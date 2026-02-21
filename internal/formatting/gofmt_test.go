package formatting

import "testing"

func TestGoSource(t *testing.T) {
	t.Parallel()

	t.Run("formats valid source", func(t *testing.T) {
		t.Parallel()

		formatted, err := GoSource("package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"x\")}\n")
		if err != nil {
			t.Fatalf("GoSource() error = %v", err)
		}
		const want = "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"x\") }\n"
		if formatted != want {
			t.Fatalf("GoSource() = %q, want %q", formatted, want)
		}
	})

	t.Run("returns error for invalid source", func(t *testing.T) {
		t.Parallel()

		if _, err := GoSource("package main\nfunc main( {\n"); err == nil {
			t.Fatal("GoSource() error = nil, want non-nil")
		}
	})

	t.Run("returns error for empty source", func(t *testing.T) {
		t.Parallel()

		if _, err := GoSource("   \n\t"); err == nil {
			t.Fatal("GoSource() error = nil, want non-nil")
		}
	})
}
