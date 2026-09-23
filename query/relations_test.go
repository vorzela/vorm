package query

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type relUser struct {
	ID    int64  `db:"id"`
	Email string `db:"email"`

	Posts []relPost `db:"-"`
	Team  *relTeam  `db:"-"`
	Tags  []relTag  `db:"-"`

	TeamID int64 `db:"team_id"`
}

type relPost struct {
	ID     int64  `db:"id"`
	UserID int64  `db:"user_id"`
	Title  string `db:"title"`

	Comments []relComment `db:"-"`
}

type relComment struct {
	ID     int64  `db:"id"`
	PostID int64  `db:"post_id"`
	Body   string `db:"body"`
}

type relTeam struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type relTag struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

var (
	relUsers    = Model[relUser](Meta{Table: "rel_users", Columns: []string{"id", "email", "team_id"}})
	relPosts    = Model[relPost](Meta{Table: "rel_posts", Columns: []string{"id", "user_id", "title"}})
	relTeams    = Model[relTeam](Meta{Table: "rel_teams", Columns: []string{"id", "name"}})
	relTags     = Model[relTag](Meta{Table: "rel_tags", Columns: []string{"id", "name"}})
	relComments = Model[relComment](Meta{Table: "rel_comments", Columns: []string{"id", "post_id", "body"}})
)

func TestLoadHasManyIssuesOneQueryForWholeBatch(t *testing.T) {
	db := (&fakeDB{}).on("rel_posts", []string{"id", "user_id", "title"},
		[]any{int64(10), int64(1), "first"},
		[]any{int64(11), int64(1), "second"},
		[]any{int64(12), int64(2), "third"},
	)
	users := []relUser{{ID: 1}, {ID: 2}, {ID: 3}}
	parents := []*relUser{&users[0], &users[1], &users[2]}

	err := LoadHasMany(context.Background(), db, parents, HasMany[relUser, relPost]{
		Related:    relPosts,
		ForeignKey: "user_id",
		ParentKey:  func(u *relUser) any { return u.ID },
		ChildKey:   func(p *relPost) any { return p.UserID },
		Assign:     func(u *relUser, ps []relPost) { u.Posts = ps },
	})
	if err != nil {
		t.Fatal(err)
	}
	if db.count() != 1 {
		t.Fatalf("expected exactly 1 query (no N+1), got %d: %v", db.count(), db.statements())
	}
	gotSQL := db.statements()[0].SQL
	wantSQL := `SELECT "id", "user_id", "title" FROM "rel_posts" WHERE "user_id" IN ($1, $2, $3)`
	if gotSQL != wantSQL {
		t.Fatalf("has-many SQL:\n got: %s\nwant: %s", gotSQL, wantSQL)
	}
	if len(users[0].Posts) != 2 || len(users[1].Posts) != 1 || len(users[2].Posts) != 0 {
		t.Fatalf("bad grouping: %d/%d/%d", len(users[0].Posts), len(users[1].Posts), len(users[2].Posts))
	}
	if got := db.statements()[0].SQL; !strings.Contains(got, "IN (") {
		t.Fatalf("expected batched IN query, got %s", got)
	}
}

func TestLoadHasManyNormalizesMixedKeyTypes(t *testing.T) {
	// Drivers report integers with different widths; grouping must still match.
	db := (&fakeDB{}).on("rel_posts", []string{"id", "user_id", "title"},
		[]any{int64(10), int32(1), "first"},
	)
	users := []relUser{{ID: 1}}
	parents := []*relUser{&users[0]}
	err := LoadHasMany(context.Background(), db, parents, HasMany[relUser, relPost]{
		Related:    relPosts,
		ForeignKey: "user_id",
		ParentKey:  func(u *relUser) any { return int32(u.ID) },
		ChildKey:   func(p *relPost) any { return p.UserID },
		Assign:     func(u *relUser, ps []relPost) { u.Posts = ps },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(users[0].Posts) != 1 {
		t.Fatalf("int32/int64 keys did not match")
	}
}

func TestLoadBelongsTo(t *testing.T) {
	db := (&fakeDB{}).on("rel_teams", []string{"id", "name"},
		[]any{int64(5), "Platform"},
	)
	users := []relUser{{ID: 1, TeamID: 5}, {ID: 2, TeamID: 5}, {ID: 3, TeamID: 9}}
	parents := []*relUser{&users[0], &users[1], &users[2]}

	err := LoadBelongsTo(context.Background(), db, parents, BelongsTo[relUser, relTeam]{
		Related:   relTeams,
		ParentKey: func(u *relUser) any { return u.TeamID },
		ChildKey:  func(tm *relTeam) any { return tm.ID },
		Assign:    func(u *relUser, tm *relTeam) { u.Team = tm },
	})
	if err != nil {
		t.Fatal(err)
	}
	if db.count() != 1 {
		t.Fatalf("expected 1 query, got %d", db.count())
	}
	if db.statements()[0].SQL != `SELECT "id", "name" FROM "rel_teams" WHERE "id" IN ($1, $2)` {
		t.Fatalf("belongs-to SQL: %s", db.statements()[0].SQL)
	}
	if users[0].Team == nil || users[0].Team.Name != "Platform" {
		t.Fatalf("owner not assigned: %+v", users[0].Team)
	}
	if users[2].Team != nil {
		t.Fatal("missing owner must stay nil")
	}
}

func TestLoadBelongsToManyUsesTwoQueries(t *testing.T) {
	db := (&fakeDB{}).
		on("rel_user_tags", []string{"user_id", "tag_id"},
			[]any{int64(1), int64(100)},
			[]any{int64(1), int64(101)},
			[]any{int64(2), int64(100)},
		).
		on("rel_tags", []string{"id", "name"},
			[]any{int64(100), "go"},
			[]any{int64(101), "sql"},
		)

	users := []relUser{{ID: 1}, {ID: 2}}
	parents := []*relUser{&users[0], &users[1]}

	err := LoadBelongsToMany(context.Background(), db, parents, BelongsToMany[relUser, relTag]{
		Related:         relTags,
		PivotTable:      "rel_user_tags",
		PivotParentKey:  "user_id",
		PivotRelatedKey: "tag_id",
		ParentKey:       func(u *relUser) any { return u.ID },
		ChildKey:        func(tg *relTag) any { return tg.ID },
		Assign:          func(u *relUser, ts []relTag) { u.Tags = ts },
	})
	if err != nil {
		t.Fatal(err)
	}
	if db.count() != 2 {
		t.Fatalf("expected 2 queries for the whole batch, got %d", db.count())
	}
	st := db.statements()
	if st[0].SQL != `SELECT "user_id", "tag_id" FROM "rel_user_tags" WHERE "user_id" IN ($1, $2)` {
		t.Fatalf("pivot SQL: %s", st[0].SQL)
	}
	if st[1].SQL != `SELECT "id", "name" FROM "rel_tags" WHERE "id" IN ($1, $2)` {
		t.Fatalf("related SQL: %s", st[1].SQL)
	}
	for _, s := range st {
		if strings.Contains(s.SQL, "*") {
			t.Fatalf("SELECT *: %s", s.SQL)
		}
	}
	if len(users[0].Tags) != 2 || len(users[1].Tags) != 1 {
		t.Fatalf("bad pivot grouping: %d/%d", len(users[0].Tags), len(users[1].Tags))
	}
}

func TestWithRunsRegisteredLoader(t *testing.T) {
	RegisterRelation(Relation{
		Name:       "posts",
		Field:      "Posts",
		Kind:       RelationHasMany,
		Table:      "rel_posts",
		LocalKey:   "id",
		ForeignKey: "user_id",
	}, func(ctx context.Context, db DB, parents []*relUser) error {
		return LoadHasMany(ctx, db, parents, HasMany[relUser, relPost]{
			Related:    relPosts,
			ForeignKey: "user_id",
			ParentKey:  func(u *relUser) any { return u.ID },
			ChildKey:   func(p *relPost) any { return p.UserID },
			Assign:     func(u *relUser, ps []relPost) { u.Posts = ps },
		})
	})

	db := (&fakeDB{}).
		on("rel_users", []string{"id", "email", "team_id"},
			[]any{int64(1), "a@x.io", int64(5)},
		).
		on("rel_posts", []string{"id", "user_id", "title"},
			[]any{int64(10), int64(1), "hello"},
		)

	got, err := relUsers.With("posts").Get(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Posts) != 1 || got[0].Posts[0].Title != "hello" {
		t.Fatalf("relation not eager-loaded: %+v", got)
	}
	if names := Relations[relUser](); len(names) == 0 || names[0].Name != "posts" {
		t.Fatalf("relation metadata missing: %+v", names)
	}
}

func TestWithUnknownRelationIsValidationError(t *testing.T) {
	db := (&fakeDB{}).on("rel_users", []string{"id", "email", "team_id"},
		[]any{int64(1), "a@x.io", int64(5)},
	)
	_, err := relUsers.With("nope").Get(context.Background(), db)
	if err == nil {
		t.Fatal("expected error for unknown relation")
	}
	if !IsValidationError(err) {
		t.Fatalf("want validation error, got %v (kind %s)", err, Classify(err))
	}
}

func TestNormalizeKey(t *testing.T) {
	if normalizeKey(int32(5)) != normalizeKey(int64(5)) {
		t.Error("int widths must normalize together")
	}
	if normalizeKey([]byte("a")) != normalizeKey("a") {
		t.Error("[]byte and string must normalize together")
	}
	if normalizeKey(nil) != nil {
		t.Error("nil stays nil")
	}
	v := 7
	if normalizeKey(&v) != normalizeKey(7) {
		t.Error("pointer must dereference")
	}
	var np *int
	if normalizeKey(np) != nil {
		t.Error("nil pointer must normalize to nil")
	}
}

func TestLoadHasManySkipsQueryWhenNoParents(t *testing.T) {
	db := &fakeDB{}
	err := LoadHasMany(context.Background(), db, nil, HasMany[relUser, relPost]{
		Related:    relPosts,
		ForeignKey: "user_id",
		ParentKey:  func(u *relUser) any { return u.ID },
		ChildKey:   func(p *relPost) any { return p.UserID },
		Assign:     func(u *relUser, ps []relPost) { u.Posts = ps },
	})
	if err != nil {
		t.Fatal(err)
	}
	if db.count() != 0 {
		t.Fatalf("expected no query, got %d", db.count())
	}
}

func TestLoadHasManyRequiresConfiguration(t *testing.T) {
	users := []relUser{{ID: 1}}
	err := LoadHasMany(context.Background(), &fakeDB{}, []*relUser{&users[0]}, HasMany[relUser, relPost]{})
	if err == nil || !IsValidationError(err) {
		t.Fatalf("want validation error, got %v", err)
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatal("expected *query.Error")
	}
}

func TestWithNestedRelation(t *testing.T) {
	RegisterRelation(Relation{
		Name:       "posts",
		Field:      "Posts",
		Kind:       RelationHasMany,
		Table:      "rel_posts",
		LocalKey:   "id",
		ForeignKey: "user_id",
	}, func(ctx context.Context, db DB, parents []*relUser) error {
		return LoadHasMany(ctx, db, parents, HasMany[relUser, relPost]{
			Related:    relPosts,
			ForeignKey: "user_id",
			ParentKey:  func(u *relUser) any { return u.ID },
			ChildKey:   func(p *relPost) any { return p.UserID },
			Assign:     func(u *relUser, ps []relPost) { u.Posts = ps },
		})
	})
	RegisterRelation(Relation{
		Name:       "comments",
		Field:      "Comments",
		Kind:       RelationHasMany,
		Table:      "rel_comments",
		LocalKey:   "id",
		ForeignKey: "post_id",
	}, func(ctx context.Context, db DB, parents []*relPost) error {
		return LoadHasMany(ctx, db, parents, HasMany[relPost, relComment]{
			Related:    relComments,
			ForeignKey: "post_id",
			ParentKey:  func(p *relPost) any { return p.ID },
			ChildKey:   func(c *relComment) any { return c.PostID },
			Assign:     func(p *relPost, cs []relComment) { p.Comments = cs },
		})
	})

	db := (&fakeDB{}).
		on("rel_users", []string{"id", "email", "team_id"},
			[]any{int64(1), "a@x.io", int64(5)},
		).
		on("rel_posts", []string{"id", "user_id", "title"},
			[]any{int64(10), int64(1), "hello"},
		).
		on("rel_comments", []string{"id", "post_id", "body"},
			[]any{int64(20), int64(10), "nice"},
		)

	got, err := relUsers.With("posts.comments").Get(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Posts) != 1 || len(got[0].Posts[0].Comments) != 1 {
		t.Fatalf("nested relation not loaded: %+v", got)
	}
	if got[0].Posts[0].Comments[0].Body != "nice" {
		t.Fatalf("comment body: %+v", got[0].Posts[0].Comments)
	}
	if db.count() != 3 {
		t.Fatalf("nested With should be 3 batched queries, got %d: %v", db.count(), db.statements())
	}
	st := db.statements()
	if st[0].SQL != `SELECT "id", "email", "team_id" FROM "rel_users"` {
		t.Fatalf("users SQL: %s", st[0].SQL)
	}
	if st[1].SQL != `SELECT "id", "user_id", "title" FROM "rel_posts" WHERE "user_id" IN ($1)` {
		t.Fatalf("posts SQL: %s", st[1].SQL)
	}
	if st[2].SQL != `SELECT "id", "post_id", "body" FROM "rel_comments" WHERE "post_id" IN ($1)` {
		t.Fatalf("comments SQL: %s", st[2].SQL)
	}
	for _, s := range st {
		if strings.Contains(s.SQL, "*") {
			t.Fatalf("SELECT *: %s", s.SQL)
		}
	}
}

type morphComment struct {
	ID              int64  `db:"id"`
	Body            string `db:"body"`
	CommentableType string `db:"commentable_type"`
	CommentableID   int64  `db:"commentable_id"`
	Commentable     any    `db:"-"`
}

type morphPost struct {
	ID    int64  `db:"id"`
	Title string `db:"title"`
}

func TestLoadMorphManyFiltersTypeAndIDs(t *testing.T) {
	Comments := Model[morphComment](Meta{
		Table:   "comments",
		Columns: []string{"id", "body", "commentable_type", "commentable_id"},
	})
	db := (&fakeDB{}).on("comments", []string{"id", "body", "commentable_type", "commentable_id"},
		[]any{int64(1), "hi", "posts", int64(10)},
		[]any{int64(2), "yo", "posts", int64(11)},
	)
	posts := []morphPost{{ID: 10}, {ID: 11}}
	parents := []*morphPost{&posts[0], &posts[1]}
	var comments [][]morphComment
	err := LoadHasMany(context.Background(), db, parents, HasMany[morphPost, morphComment]{
		Related:    Comments,
		ForeignKey: "commentable_id",
		ParentKey:  func(p *morphPost) any { return p.ID },
		ChildKey:   func(c *morphComment) any { return c.CommentableID },
		Assign: func(p *morphPost, rows []morphComment) {
			comments = append(comments, rows)
		},
		Modify: func(b *Builder[morphComment]) *Builder[morphComment] {
			return b.Where("commentable_type", "posts")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0]
	want := `SELECT "id", "body", "commentable_type", "commentable_id" FROM "comments" WHERE "commentable_id" IN ($1, $2) AND "commentable_type" = $3`
	if got.SQL != want {
		t.Fatalf("morphMany SQL:\n got: %s\nwant: %s", got.SQL, want)
	}
	if len(got.Args) != 3 || got.Args[2] != "posts" {
		t.Fatalf("args=%v (type column must bind the table name)", got.Args)
	}
	if len(comments) != 2 || len(comments[0]) != 1 || len(comments[1]) != 1 {
		t.Fatalf("grouping: %+v", comments)
	}
}

func TestLoadMorphToUsesTableNameAndBatchedIN(t *testing.T) {
	Posts := Model[morphPost](Meta{
		Table:   "posts",
		Columns: []string{"id", "title"},
	})
	db := (&fakeDB{}).on("posts", []string{"id", "title"},
		[]any{int64(10), "hello"},
	)
	rows := []morphComment{
		{ID: 1, CommentableType: "posts", CommentableID: 10},
		{ID: 2, CommentableType: "posts", CommentableID: 10},
	}
	parents := []*morphComment{&rows[0], &rows[1]}
	err := LoadMorphTo(context.Background(), db, parents, MorphTo[morphComment]{
		ParentType: func(c *morphComment) string { return c.CommentableType },
		ParentID:   func(c *morphComment) any { return c.CommentableID },
		Assign:     func(c *morphComment, v any) { c.Commentable = v },
		Targets: map[string]MorphLoader{
			"posts": MorphLoad(Posts, func(p *morphPost) any { return p.ID }),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if db.count() != 1 {
		t.Fatalf("one query per morph type, got %d: %v", db.count(), db.statements())
	}
	got := db.statements()[0]
	if got.SQL != `SELECT "id", "title" FROM "posts" WHERE "id" IN ($1)` {
		t.Fatalf("morphTo SQL: %s", got.SQL)
	}
	if got.Args[0] != int64(10) {
		t.Fatalf("args=%v", got.Args)
	}
	p0, ok := rows[0].Commentable.(*morphPost)
	if !ok || p0.Title != "hello" {
		t.Fatalf("assigned: %+v", rows[0].Commentable)
	}
	if rows[1].Commentable != rows[0].Commentable {
		t.Fatal("same parent id should resolve to the same row")
	}
}

func TestLoadHasManyThroughSQL(t *testing.T) {
	db := (&fakeDB{}).on("rel_comments", []string{"id", "post_id", "body", "user_id"},
		[]any{int64(1), int64(10), "hi", int64(1)},
		[]any{int64(2), int64(11), "yo", int64(2)},
	)
	users := []relUser{{ID: 1}, {ID: 2}}
	var comments [][]relComment
	err := LoadHasManyThrough(context.Background(), db, []*relUser{&users[0], &users[1]}, HasManyThrough[relUser, relComment]{
		Related:      relComments,
		ThroughTable: "rel_posts",
		ThroughLocal: "user_id",
		ThroughFar:   "id",
		FarKey:       "post_id",
		ParentKey:    func(u *relUser) any { return u.ID },
		Assign: func(u *relUser, rows []relComment) {
			comments = append(comments, rows)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	want := `SELECT "rel_comments"."id", "rel_comments"."post_id", "rel_comments"."body", "rel_posts"."user_id" FROM "rel_comments" INNER JOIN "rel_posts" ON "rel_posts"."id" = "rel_comments"."post_id" WHERE "rel_posts"."user_id" IN ($1, $2)`
	if got != want {
		t.Fatalf("through SQL:\n got: %s\nwant: %s", got, want)
	}
	if strings.Contains(got, "*") {
		t.Fatal(got)
	}
	if len(comments) != 2 || len(comments[0]) != 1 || comments[0][0].Body != "hi" {
		t.Fatalf("grouped: %+v", comments)
	}
}

func TestLoadHasOneOfManyDistinctOnSQL(t *testing.T) {
	type ofManyPost struct {
		ID        int64  `db:"id"`
		UserID    int64  `db:"user_id"`
		Title     string `db:"title"`
		CreatedAt string `db:"created_at"`
	}
	Posts := Model[ofManyPost](Meta{
		Table: "posts", Columns: []string{"id", "user_id", "title", "created_at"}, PrimaryKey: "id",
	})
	db := (&fakeDB{}).on("posts", []string{"id", "user_id", "title", "created_at"},
		[]any{int64(2), int64(1), "latest", "2024-02-01"},
	)
	users := []relUser{{ID: 1}, {ID: 2}}
	var assigned int
	err := LoadHasOneOfMany(context.Background(), db, []*relUser{&users[0], &users[1]}, HasOneOfMany[relUser, ofManyPost]{
		Related:    Posts,
		ForeignKey: "user_id",
		OrderCol:   "created_at",
		Desc:       true,
		ParentKey:  func(u *relUser) any { return u.ID },
		ChildKey:   func(p *ofManyPost) any { return p.UserID },
		Assign:     func(u *relUser, p *ofManyPost) { assigned++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	want := `SELECT DISTINCT ON ("user_id") "id", "user_id", "title", "created_at" FROM "posts" WHERE "user_id" IN ($1, $2) ORDER BY "user_id" ASC, "created_at" DESC`
	if got != want {
		t.Fatalf("of-many SQL:\n got: %s\nwant: %s", got, want)
	}
	if assigned != 1 {
		t.Fatalf("assigned=%d", assigned)
	}
}

func TestLoadHasOneOfManyMySQLMaxJoin(t *testing.T) {
	type ofManyPost struct {
		ID        int64  `db:"id"`
		UserID    int64  `db:"user_id"`
		CreatedAt string `db:"created_at"`
	}
	Posts := Model[ofManyPost](Meta{
		Table: "posts", Columns: []string{"id", "user_id", "created_at"}, PrimaryKey: "id",
	})
	db := (&fakeDB{}).on("posts", []string{"id", "user_id", "created_at"},
		[]any{int64(2), int64(1), "2024-02-01"},
	)
	_, err := loadOfManyMySQL(context.Background(), db, Posts, "user_id", "created_at", true, []any{int64(1), int64(2)})
	if err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	if !strings.Contains(got, "MAX(`created_at`)") || !strings.Contains(got, "INNER JOIN") {
		t.Fatalf("mysql of-many: %s", got)
	}
	if strings.Contains(got, "*") {
		t.Fatal(got)
	}
}

func TestLoadMorphToManyFiltersType(t *testing.T) {
	db := (&fakeDB{}).
		on("taggables", []string{"taggable_id", "tag_id"},
			[]any{int64(1), int64(100)},
		).
		on("rel_tags", []string{"id", "name"},
			[]any{int64(100), "go"},
		)
	users := []relUser{{ID: 1}}
	err := LoadBelongsToMany(context.Background(), db, []*relUser{&users[0]}, BelongsToMany[relUser, relTag]{
		Related:         relTags,
		PivotTable:      "taggables",
		PivotParentKey:  "taggable_id",
		PivotRelatedKey: "tag_id",
		MorphType:       "users",
		MorphTypeColumn: "taggable_type",
		ParentKey:       func(u *relUser) any { return u.ID },
		ChildKey:        func(tg *relTag) any { return tg.ID },
		Assign:          func(u *relUser, ts []relTag) { u.Tags = ts },
	})
	if err != nil {
		t.Fatal(err)
	}
	got := db.statements()[0].SQL
	want := `SELECT "taggable_id", "tag_id" FROM "taggables" WHERE "taggable_id" IN ($1) AND "taggable_type" = $2`
	if got != want {
		t.Fatalf("morph pivot:\n got: %s\nwant: %s", got, want)
	}
	if db.statements()[0].Args[1] != "users" {
		t.Fatalf("type arg=%v", db.statements()[0].Args)
	}
}
