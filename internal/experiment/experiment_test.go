//go:build experiment

package experiment_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment/modes"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

func testSource(t *testing.T) config.Source {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	src, err := config.NewSourceFromDir(filepath.Join(wd, "..", "..", "testdata", "configs"))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func loadEvtTypes(t *testing.T, src config.Source) map[string]*event.EventTypeConfig {
	t.Helper()
	raw, err := src.LoadAllEventConfigs()
	if err != nil {
		t.Fatal(err)
	}
	m := make(map[string]*event.EventTypeConfig)
	for _, data := range raw {
		var cfg event.EventTypeConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatal(err)
		}
		m[cfg.Name] = &cfg
	}
	return m
}

func allModes(t *testing.T) []experiment.ModeEntry {
	t.Helper()
	src := testSource(t)
	btReg := bt.DefaultRegistry()
	hybrid, err := modes.NewHybridNPC("hybrid", event.Vec3{}, src, btReg)
	if err != nil {
		t.Fatal(err)
	}
	fsmDC, err := modes.NewFSMDCNPC("fsmdc", event.Vec3{}, src)
	if err != nil {
		t.Fatal(err)
	}
	btDC, err := modes.NewBTDCNPC("btdc", event.Vec3{}, src, btReg)
	if err != nil {
		t.Fatal(err)
	}
	pureFSM, err := modes.NewPureFSMNPC("purefsm", src)
	if err != nil {
		t.Fatal(err)
	}
	pureBT, err := modes.NewPureBTNPC("purebt", src, btReg)
	if err != nil {
		t.Fatal(err)
	}
	return []experiment.ModeEntry{
		{Name: "Hybrid", NPC: hybrid},
		{Name: "FSM+DC", NPC: fsmDC},
		{Name: "BT+DC", NPC: btDC},
		{Name: "PureFSM", NPC: pureFSM},
		{Name: "PureBT", NPC: pureBT},
	}
}

func runScenario(t *testing.T, s *experiment.Scenario) *experiment.ComparisonReport {
	t.Helper()
	src := testSource(t)
	evtTypes := loadEvtTypes(t, src)
	return experiment.NewRunner(s, evtTypes).RunAll(allModes(t))
}

// --- 表 1：三层不可替代性 ---

func TestExperiment_DistanceTrap(t *testing.T) {
	r := runScenario(t, experiment.ScenarioDistanceTrap())
	for _, m := range r.Results {
		experiment.PrintModeDetail(t, m)
	}
	r.PrintTable(t)
	for _, name := range []string{"Hybrid", "FSM+DC", "BT+DC"} {
		if m := r.Get(name); m != nil && m.Correctness < 100 {
			t.Errorf("[R4] %s should pass distance trap, got %.1f%%", name, m.Correctness)
		}
	}
	for _, name := range []string{"PureFSM", "PureBT"} {
		if m := r.Get(name); m != nil && m.Correctness >= 100 {
			t.Errorf("[R4] %s should fail distance trap, got %.1f%%", name, m.Correctness)
		}
	}
}

func TestExperiment_MultiStepBehavior(t *testing.T) {
	r := runScenario(t, experiment.ScenarioMultiStepBehavior())
	for _, m := range r.Results {
		experiment.PrintModeDetail(t, m)
	}
	r.PrintTable(t)
	hybrid := r.Get("Hybrid")
	if hybrid != nil && len(hybrid.BBCheckResults) > 0 && !hybrid.BBCheckResults[0].Pass {
		t.Errorf("[R5] Hybrid current_action should be 'run_away', got '%s'", hybrid.BBCheckResults[0].Actual)
	}
	fsmDC := r.Get("FSM+DC")
	if fsmDC != nil && len(fsmDC.BBCheckResults) > 0 && fsmDC.BBCheckResults[0].Pass {
		t.Errorf("[R5] FSM+DC should NOT have current_action (no BT)")
	}
}

func TestExperiment_StateLifecycle(t *testing.T) {
	r := runScenario(t, experiment.ScenarioStateLifecycle())
	for _, m := range r.Results {
		experiment.PrintModeDetail(t, m)
	}
	r.PrintTable(t)
	for _, name := range []string{"Hybrid", "FSM+DC"} {
		m := r.Get(name)
		if m == nil {
			continue
		}
		for _, chk := range m.BBCheckResults {
			if chk.Key == "exit_cleanup_done" && !chk.Pass {
				t.Errorf("[R6] %s exit_cleanup_done should be set, got '%s'", name, chk.Actual)
			}
		}
	}
	btDC := r.Get("BT+DC")
	if btDC != nil {
		for _, chk := range btDC.BBCheckResults {
			if chk.Key == "exit_cleanup_done" && chk.Pass {
				t.Errorf("[R6] BT+DC should NOT have exit_cleanup_done (no FSM)")
			}
		}
	}
}

func TestExperiment_Civilian3Events(t *testing.T) {
	r := runScenario(t, experiment.ScenarioCivilian3Events())
	for _, m := range r.Results {
		experiment.PrintModeDetail(t, m)
	}
	r.PrintTable(t)
}
