package menuet

import "testing"

var testProject = "caseymrm/whyawake"
var testOldVersion = "v0.3"

func TestCheckForRelease(t *testing.T) {
	CheckForRestart()
	release := CheckForNewRelease(testProject, testOldVersion)
	if release == nil {
		t.Fail()
	}
	if release.TagName != "v0.4" {
		t.Errorf("Unexpected release: %+v", release)
	}
}

func TestUpdateInPlace(t *testing.T) {
	CheckForRestart()
	release := CheckForNewRelease(testProject, testOldVersion)
	if release == nil {
		t.Fail()
	}
	err := UpdateApp(release)
	t.Errorf("UpdateApp: %+v", err)

}
