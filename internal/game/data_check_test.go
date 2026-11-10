package game

import "testing"

func TestValidateDataRealFiles(t *testing.T) {
	useContentTables(t)
	if err := ValidateData(); err != nil {
		t.Fatal(err)
	}
}
