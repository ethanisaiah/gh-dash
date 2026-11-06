package data

import (
	"encoding/json"
	"testing"
	"time"
)

const projectFixture = `{
  "id": "PVT_1", "title": "Astrum", "number": 1,
  "fields": {"nodes": [
    {"__typename": "ProjectV2Field", "id": "F_title", "name": "Title"},
    {"__typename": "ProjectV2SingleSelectField", "id": "F_status", "name": "Status",
     "options": [{"id": "o1", "name": "Backlog"}, {"id": "o3", "name": "Ready to Build"}]},
    {"__typename": "ProjectV2IterationField", "id": "F_iter", "name": "Iteration",
     "configuration": {
       "iterations": [
         {"id": "i1", "title": "Week of Nov 5", "startDate": "2026-11-05", "duration": 7},
         {"id": "i2", "title": "Week of Nov 12", "startDate": "2026-11-12", "duration": 7}],
       "completedIterations": [
         {"id": "i0", "title": "Week of Oct 29", "startDate": "2026-10-29", "duration": 7}]}}
  ]}
}`

func loadProjectFixture(t *testing.T) *Project {
	t.Helper()
	var p Project
	if err := json.Unmarshal([]byte(projectFixture), &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func TestProjectFieldLookupByName(t *testing.T) {
	p := loadProjectFixture(t)

	status := p.FieldByName("status")
	if status == nil || status.Id != "F_status" || !status.IsSingleSelect() {
		t.Fatalf("status field not found by case-insensitive name: %+v", status)
	}
	if opt := status.OptionByName(" ready to build "); opt == nil || opt.Id != "o3" {
		t.Fatalf("option lookup failed: %+v", opt)
	}
	if status.OptionByName("Merged") != nil {
		t.Fatal("unknown option should not match")
	}
	if p.FieldByName("Priority") != nil {
		t.Fatal("missing field should be nil")
	}
	if !p.FieldByName("Iteration").IsIteration() {
		t.Fatal("iteration field type not detected")
	}
}

func TestIterations(t *testing.T) {
	f := loadProjectFixture(t).FieldByName("Iteration")

	cases := []struct {
		day  string
		want string
	}{
		{"2026-11-05", "i1"}, // Thursday, first day
		{"2026-11-11", "i1"}, // Wednesday, last day
		{"2026-11-12", "i2"},
		{"2026-11-30", ""},
	}
	for _, c := range cases {
		now, _ := time.ParseInLocation("2006-01-02", c.day, time.Local)
		now = now.Add(15 * time.Hour)
		got := f.CurrentIteration(now)
		if (got == nil && c.want != "") || (got != nil && got.Id != c.want) {
			t.Errorf("%s: got %+v, want %q", c.day, got, c.want)
		}
	}

	if it := f.IterationByTitle("week of oct 29"); it == nil || it.Id != "i0" {
		t.Errorf("completed iteration lookup failed: %+v", it)
	}
	if n := len(f.ActiveIterations()); n != 2 {
		t.Errorf("want 2 active iterations, got %d", n)
	}
}

func TestProjectItemCache(t *testing.T) {
	if _, ok := GetProjectItem("I_unknown"); ok {
		t.Fatal("unfetched issue should report !ok")
	}

	var node projectItemNode
	_ = json.Unmarshal([]byte(`{"id": "PVTI_1", "project": {"id": "PVT_1"},
	  "status": {"optionId": "o1", "name": "Backlog"}, "iteration": {}}`), &node)
	item := node.toProjectItem()
	if item.StatusName() != "Backlog" || item.Iteration != nil {
		t.Fatalf("unexpected item: %+v", item)
	}

	SetProjectItem("I_1", &item)
	got, ok := GetProjectItem("I_1")
	if !ok || got == nil || got.ItemId != "PVTI_1" {
		t.Fatalf("cache roundtrip failed: %+v %v", got, ok)
	}

	SetProjectItem("I_2", nil)
	got, ok = GetProjectItem("I_2")
	if !ok || got != nil {
		t.Fatal("issue fetched but not in project should be ok && nil")
	}
}
