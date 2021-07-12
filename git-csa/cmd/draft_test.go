package cmd

import "testing"

func Test_insertIssueLink(t *testing.T) {
	table := []struct {
		input    string
		expected string
	}{
		{
			input:    "",
			expected: "issue: cybozu/csa#1",
		},
		{
			input:    "issue: cybozu/csa#1",
			expected: "issue: cybozu/csa#1",
		},
		{
			input: "Signed-off-by: foo",
			expected: `issue: cybozu/csa#1
Signed-off-by: foo`,
		},
		{
			input: "message",
			expected: `message
issue: cybozu/csa#1`,
		},
		{
			input: `message
`,
			expected: `message
issue: cybozu/csa#1`,
		},
		{
			input: `message

Signed-off-by: foo`,
			expected: `message

issue: cybozu/csa#1
Signed-off-by: foo`,
		},
	}

	for i, tt := range table {
		actual := insertIssueLink(tt.input, "cybozu", "csa", 1)
		if actual != tt.expected {
			t.Fatalf("[%d] expect: %s, but actual: %s,", i, tt.expected, actual)
		}
	}
}
