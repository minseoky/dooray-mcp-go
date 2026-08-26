package dooray

import "testing"

func TestContentDispositionFileName(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{`attachment; filename*=UTF-8''%ED%95%9C%EA%B8%80.png`, "한글.png"},
		{`attachment; filename="report v2.pdf"`, "report v2.pdf"},
		{`attachment; filename=plain.txt`, "plain.txt"},
		{`attachment; filename=plain.txt; filename*=UTF-8''encoded.txt`, "encoded.txt"},
		{"", ""},
		{"inline", ""},
	}

	for _, testCase := range cases {
		if got := contentDispositionFileName(testCase.header); got != testCase.want {
			t.Errorf("contentDispositionFileName(%q) = %q, want %q", testCase.header, got, testCase.want)
		}
	}
}

func TestSanitizeFileName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"report.pdf", "report.pdf"},
		{"../../etc/passwd", "passwd"},
		{`C:\Users\me\secret.txt`, "secret.txt"},
		{"a:b*c?d\"e<f>g|h.png", "a_b_c_d_e_f_g_h.png"},
		{"", "dooray-attachment"},
		{"...", "dooray-attachment"},
		{"/", "dooray-attachment"},
	}

	for _, testCase := range cases {
		if got := sanitizeFileName(testCase.input); got != testCase.want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", testCase.input, got, testCase.want)
		}
	}
}

func TestEscapeComponent(t *testing.T) {
	cases := map[string]string{
		"3175089685857117362": "3175089685857117362",
		"a/b":                 "a%2Fb",
		"한":                   "%ED%95%9C",
		"a b+c":               "a%20b%2Bc",
		"-_.!~*'()":           "-_.!~*'()",
	}

	for input, want := range cases {
		if got := EscapeComponent(input); got != want {
			t.Errorf("EscapeComponent(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestShouldSendDownloadAuthorization(t *testing.T) {
	cases := []struct {
		target string
		want   bool
	}{
		{"https://api.dooray.com/project/v1/x", true},
		{"https://file-api.dooray.com/download/abc", true},
		{"http://file-api.dooray.com/download/abc", false},
		{"https://evil.example.com/download/abc", false},
	}

	for _, testCase := range cases {
		target := mustParse(t, testCase.target)
		if got := shouldSendDownloadAuthorization(target, "https://api.dooray.com"); got != testCase.want {
			t.Errorf("shouldSendDownloadAuthorization(%q) = %v, want %v", testCase.target, got, testCase.want)
		}
	}
}
