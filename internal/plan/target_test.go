package plan

import "testing"

func TestParseTarget(t *testing.T) {
	tests := []struct {
		in      string
		want    Target
		wantErr bool
	}{
		{"https://example.com", Target{Raw: "https://example.com", Scheme: "https", Host: "example.com", Port: 443}, false},
		{"http://example.com:8080/x", Target{Raw: "http://example.com:8080/x", Scheme: "http", Host: "example.com", Port: 8080}, false},
		{"example.com:22", Target{Raw: "example.com:22", Scheme: "tcp", Host: "example.com", Port: 22}, false},
		{"example.com", Target{Raw: "example.com", Scheme: "https", Host: "example.com", Port: 443}, false},
		{"1.1.1.1", Target{Raw: "1.1.1.1", Scheme: "https", Host: "1.1.1.1", Port: 443, IsIP: true}, false},
		{"[2001:db8::1]:443", Target{Raw: "[2001:db8::1]:443", Scheme: "tcp", Host: "2001:db8::1", Port: 443, IsIP: true}, false},
		{"2001:db8::1", Target{Raw: "2001:db8::1", Scheme: "https", Host: "2001:db8::1", Port: 443, IsIP: true}, false},
		{"", Target{}, true},
		{"ftp://example.com", Target{}, true},
		{"example.com:99999", Target{}, true},
		{"example.com:abc", Target{}, true},
	}

	for _, tc := range tests {
		got, err := ParseTarget(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseTarget(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseTarget(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}
