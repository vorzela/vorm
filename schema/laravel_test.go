package schema_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vorzela/vorm/schema"
)

func TestUsersBlueprintLikeLaravel(t *testing.T) {
	bp := schema.NewBlueprint("users")
	bp.ID()
	bp.String("first_name")
	bp.String("last_name")
	bp.String("email").Unique()
	bp.String("password")
	bp.Boolean("active").Default(true)
	bp.Integer("age").Nullable()
	bp.Timestamps()
	bp.SoftDeletes()
	if err := schema.ValidateBlueprint(bp); err != nil {
		t.Fatal(err)
	}
	up, down := bp.Compile("postgres")
	for _, want := range []string{
		"first_name VARCHAR(255) NOT NULL",
		"email VARCHAR(255) NOT NULL UNIQUE",
		"active BOOLEAN NOT NULL DEFAULT TRUE",
		"age INTEGER NULL",
		"DROP TABLE IF EXISTS users CASCADE",
	} {
		if !strings.Contains(up, want) && !strings.Contains(down, want) {
			t.Fatalf("missing %q\nup:\n%s", want, up)
		}
	}
}

func TestForeignKeyOnDelete(t *testing.T) {
	bp := schema.NewBlueprint("posts")
	bp.ID()
	bp.ForeignId("user_id").Constrained("users").CascadeOnDelete()
	up, _ := bp.Compile("postgres")
	if !strings.Contains(up, "REFERENCES users(id) ON DELETE CASCADE") {
		t.Fatal(up)
	}
	bp2 := schema.NewBlueprint("posts")
	bp2.ForeignId("user_id").Constrained("users").NullOnDelete()
	up2, _ := bp2.Compile("postgres")
	if !strings.Contains(up2, "ON DELETE SET NULL") || !strings.Contains(up2, "user_id BIGINT NULL") {
		t.Fatal(up2)
	}
}

func TestCompositePrimaryKey(t *testing.T) {
	bp := schema.NewBlueprint("group_members")
	bp.BelongsTo("group_id", "groups")
	bp.BelongsTo("user_id", "users")
	bp.String("role").Default("member")
	bp.Primary("group_id", "user_id")
	if err := schema.ValidateBlueprint(bp); err != nil {
		t.Fatal(err)
	}
	for _, dialect := range []string{"postgres", "mysql", "mariadb"} {
		up, _ := bp.Compile(dialect)
		if !strings.Contains(up, "PRIMARY KEY (group_id, user_id)") {
			t.Fatalf("%s missing composite PK:\n%s", dialect, up)
		}
		if strings.Contains(up, "BIGSERIAL PRIMARY KEY") || strings.Contains(up, "AUTO_INCREMENT PRIMARY KEY") {
			t.Fatalf("%s should not have surrogate id PK:\n%s", dialect, up)
		}
	}
}

func TestPrimaryConflictsWithID(t *testing.T) {
	bp := schema.NewBlueprint("group_members")
	bp.ID()
	bp.BelongsTo("group_id", "groups")
	bp.BelongsTo("user_id", "users")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when Primary follows ID")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "primary key already") {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	bp.Primary("group_id", "user_id")
}

func TestIDConflictsWithPrimary(t *testing.T) {
	bp := schema.NewBlueprint("group_members")
	bp.BelongsTo("group_id", "groups")
	bp.BelongsTo("user_id", "users")
	bp.Primary("group_id", "user_id")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when ID follows Primary")
		}
	}()
	bp.ID()
}

func TestDefaultAutoQuotesStringsAndJSON(t *testing.T) {
	bp := schema.NewBlueprint("groups")
	bp.String("currency_code").Default("KES")
	bp.Json("meta").Default("{}")
	bp.Json("settings").Default(map[string]any{"theme": "dark"})
	bp.String("role").Default("'admin'") // already quoted — left as-is
	bp.Boolean("active").Default(true)
	bp.Timestamp("created_at").DefaultRaw("CURRENT_TIMESTAMP")
	up, _ := bp.Compile("postgres")
	for _, want := range []string{
		"currency_code VARCHAR(255) NOT NULL DEFAULT 'KES'",
		`meta JSONB NULL DEFAULT '{}'`,
		`settings JSONB NULL DEFAULT '{"theme":"dark"}'`,
		"role VARCHAR(255) NOT NULL DEFAULT 'admin'",
		"active BOOLEAN NOT NULL DEFAULT TRUE",
		"created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP",
	} {
		if !strings.Contains(up, want) {
			t.Fatalf("missing %q\nup:\n%s", want, up)
		}
	}
}

func TestBelongsToOptionalParentColumn(t *testing.T) {
	bp := schema.NewBlueprint("posts")
	bp.BelongsTo("author_id", "uuid", "users")
	up, _ := bp.Compile("postgres")
	if !strings.Contains(up, "author_id BIGINT NOT NULL REFERENCES users(uuid) ON DELETE CASCADE") {
		t.Fatal(up)
	}

	bp2 := schema.NewBlueprint("profiles")
	bp2.HasOne("user_id", "users")
	up2, _ := bp2.Compile("postgres")
	if !strings.Contains(up2, "REFERENCES users(id)") {
		t.Fatal(up2)
	}
	if !strings.Contains(up2, "UNIQUE") && !strings.Contains(up2, "uq_profiles_user_id") {
		t.Fatal(up2)
	}

	bp3 := schema.NewBlueprint("posts")
	bp3.ForeignId("user_id").Constrained("users", "uuid")
	up3, _ := bp3.Compile("postgres")
	if !strings.Contains(up3, "REFERENCES users(uuid)") {
		t.Fatal(up3)
	}
}
