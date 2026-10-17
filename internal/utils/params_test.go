package utils_test

import (
	"fmt"
	"net/url"
	"testing"

	"go-core/internal/testutil"
	"go-core/internal/utils"
)

func TestParseID(t *testing.T) {
	for _, raw := range []string{"abc", "0", "-1", "1.5", "99999999999999999999", ""} {
		_, err := utils.ParseID(raw, "employee")
		testutil.AssertAppError(t, err, utils.KindValidation, fmt.Sprintf("invalid employee id: %q", raw))
	}
	if id, err := utils.ParseID("42", "department"); err != nil || id != 42 {
		t.Fatalf("%d %v", id, err)
	}
}

func TestParsePagination(t *testing.T) {
	cases := []struct {
		query         string
		limit, offset int
		wantMsg       string
	}{
		{"", 10, 0, ""},
		{"limit=100&offset=5", 100, 5, ""},
		{"limit=0", 0, 0, "limit must be between 1 and 100"},
		{"limit=101", 0, 0, "limit must be between 1 and 100"},
		{"limit=x", 0, 0, "limit must be between 1 and 100"},
		{"limit=10abc", 0, 0, "limit must be between 1 and 100"},
		{"offset=-5", 0, 0, "offset must be a non-negative integer"},
		{"offset=x", 0, 0, "offset must be a non-negative integer"},
	}
	for _, tc := range cases {
		q, _ := url.ParseQuery(tc.query)
		l, o, err := utils.ParsePagination(q)
		if tc.wantMsg != "" {
			testutil.AssertAppError(t, err, utils.KindValidation, tc.wantMsg)
			continue
		}
		if err != nil || l != tc.limit || o != tc.offset {
			t.Errorf("%q: %d %d %v", tc.query, l, o, err)
		}
	}
}

func TestParseOptionalID(t *testing.T) {
	q, _ := url.ParseQuery("departmentId=3")
	if p, err := utils.ParseOptionalID(q, "departmentId"); err != nil || p == nil || *p != 3 {
		t.Fatal(p, err)
	}
	if p, err := utils.ParseOptionalID(url.Values{}, "departmentId"); err != nil || p != nil {
		t.Fatal(p, err)
	}
	for _, raw := range []string{"x", "0"} {
		_, err := utils.ParseOptionalID(url.Values{"departmentId": {raw}}, "departmentId")
		testutil.AssertAppError(t, err, utils.KindValidation, fmt.Sprintf("invalid departmentId: %q", raw))
	}
}
