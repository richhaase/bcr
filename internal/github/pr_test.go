package github

import "testing"

func TestParseCheckState(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantState string
		wantRaw   int
	}{
		{
			name: "success",
			output: "NAME  DESCRIPTION STATE VIEW\n" +
				"ci/unit  CI Steps success nil\n" +
				"ci/lint  Lint  success nil\n",
			wantState: "success",
			wantRaw:   3,
		},
		{
			name: "failure",
			output: "NAME  DESCRIPTION STATE VIEW\n" +
				"ci/build  Build  failure nil\n",
			wantState: "failure",
			wantRaw:   2,
		},
		{
			name: "pending",
			output: "NAME  DESCRIPTION STATE VIEW\n" +
				"ci/test  Tests  pending nil\n",
			wantState: "pending",
			wantRaw:   2,
		},
		{
			name:      "empty",
			output:    "",
			wantState: "unknown",
			wantRaw:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, raw := ParseCheckState(tc.output)
			if state != tc.wantState {
				t.Errorf("ParseCheckState() state = %q, want %q", state, tc.wantState)
			}
			if raw != tc.wantRaw {
				t.Errorf("ParseCheckState() raw = %d, want %d", raw, tc.wantRaw)
			}
		})
	}
}

func TestValidReviewEvent(t *testing.T) {
	valid := []string{"request-changes", "comment", "approve"}
	for _, e := range valid {
		if !validReviewEvent(e) {
			t.Errorf("expected %q to be a valid review event", e)
		}
	}

	invalid := []string{"", "Approve", "reject", "lgtm", "request_changes"}
	for _, e := range invalid {
		if validReviewEvent(e) {
			t.Errorf("expected %q to be an invalid review event", e)
		}
	}
}
