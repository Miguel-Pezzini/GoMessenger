package mongo

import "testing"

func TestNormalizeLocalURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "adds direct connection for localhost",
			in:   "mongodb://localhost:27020",
			want: "mongodb://localhost:27020/?directConnection=true",
		},
		{
			name: "adds direct connection for localhost with path",
			in:   "mongodb://localhost:27020/friends_db",
			want: "mongodb://localhost:27020/friends_db?directConnection=true",
		},
		{
			name: "preserves existing direct connection flag",
			in:   "mongodb://localhost:27020?directConnection=true",
			want: "mongodb://localhost:27020?directConnection=true",
		},
		{
			name: "preserves remote host uri",
			in:   "mongodb://mongo_friends:27017/?appName=friends",
			want: "mongodb://mongo_friends:27017/?appName=friends",
		},
		{
			name: "preserves multi host uri",
			in:   "mongodb://localhost:27020,mongo_friends:27017",
			want: "mongodb://localhost:27020,mongo_friends:27017",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeLocalURI(tt.in); got != tt.want {
				t.Fatalf("normalizeLocalURI(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
