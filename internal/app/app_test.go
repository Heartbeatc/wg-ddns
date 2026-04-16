package app

import (
	"bytes"
	"strings"
	"testing"

	"wg-ddns/internal/model"
)

func TestInferRunContextDetectsEntryNode(t *testing.T) {
	project := model.Project{
		Nodes: model.Nodes{
			US: model.Node{Host: "1.2.3.4"},
			HK: model.Node{Host: "5.6.7.8"},
		},
	}
	var out bytes.Buffer

	rc := inferRunContextWithDetector(&out, project, model.RunContext{}, func() (string, error) {
		return "1.2.3.4", nil
	})

	if !rc.EntryIsLocal || rc.ExitIsLocal {
		t.Fatalf("rc = %+v, want entry local only", rc)
	}
	if !strings.Contains(out.String(), "--local-entry") {
		t.Fatalf("expected auto local-entry message, got %q", out.String())
	}
}

func TestInferRunContextDetectsExitNode(t *testing.T) {
	project := model.Project{
		Nodes: model.Nodes{
			US: model.Node{Host: "1.2.3.4"},
			HK: model.Node{Host: "5.6.7.8"},
		},
	}

	rc := inferRunContextWithDetector(&bytes.Buffer{}, project, model.RunContext{}, func() (string, error) {
		return "5.6.7.8", nil
	})

	if rc.EntryIsLocal || !rc.ExitIsLocal {
		t.Fatalf("rc = %+v, want exit local only", rc)
	}
}

func TestInferRunContextKeepsExplicitFlags(t *testing.T) {
	project := model.Project{
		Nodes: model.Nodes{
			US: model.Node{Host: "1.2.3.4"},
			HK: model.Node{Host: "5.6.7.8"},
		},
	}

	rc := inferRunContextWithDetector(&bytes.Buffer{}, project, model.RunContext{EntryIsLocal: true}, func() (string, error) {
		return "9.9.9.9", nil
	})

	if !rc.EntryIsLocal || rc.ExitIsLocal {
		t.Fatalf("rc = %+v, want explicit entry local preserved", rc)
	}
}
