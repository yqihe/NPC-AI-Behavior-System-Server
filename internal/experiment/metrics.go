//go:build experiment

package experiment

import (
	"fmt"
	"strings"
	"testing"
)

// TickRecord 单个 Tick 的记录
type TickRecord struct {
	Tick         int
	State        string
	ThreatLevel  float64
	EventArrived string
	Transitioned bool
}

// BBCheckResult 单个 BB 检查点的结果
type BBCheckResult struct {
	Key      string
	Expected string
	Actual   string
	Pass     bool
}

// ModeResult 单个模式的实验结果
type ModeResult struct {
	ModeName string
	Records  []TickRecord

	// M1: 行为正确性
	TotalChecks   int
	CorrectChecks int
	Correctness   float64

	// M2: 响应时延
	ResponseTicks []int
	AvgResponse   float64
	MaxResponse   int

	// M5: 行为表达力
	PreemptionOK  bool
	ArbitrationOK bool
	RecoveryOK    bool

	// BB 检查点
	BBCheckResults []BBCheckResult
}

// CalcMetrics 计算 M1/M2 指标
func (r *ModeResult) CalcMetrics(expected []ExpectedState) {
	r.TotalChecks = len(expected)
	r.CorrectChecks = 0
	for _, exp := range expected {
		if exp.AtTick < len(r.Records) && r.Records[exp.AtTick].State == exp.ExpectedState {
			r.CorrectChecks++
		}
	}
	if r.TotalChecks > 0 {
		r.Correctness = float64(r.CorrectChecks) / float64(r.TotalChecks) * 100
	}

	r.ResponseTicks = calcResponseTicks(r.Records)
	if len(r.ResponseTicks) > 0 {
		sum := 0
		for _, t := range r.ResponseTicks {
			sum += t
			if t > r.MaxResponse {
				r.MaxResponse = t
			}
		}
		r.AvgResponse = float64(sum) / float64(len(r.ResponseTicks))
	}
}

func calcResponseTicks(records []TickRecord) []int {
	var ticks []int
	for i, rec := range records {
		if rec.EventArrived != "" {
			for j := i; j < len(records); j++ {
				if records[j].Transitioned {
					ticks = append(ticks, j-i)
					break
				}
				if j == len(records)-1 {
					ticks = append(ticks, j-i)
				}
			}
		}
	}
	return ticks
}

// ComparisonReport 多模式对比报告
type ComparisonReport struct {
	Scenario string
	Results  []*ModeResult
}

// Get 按模式名获取结果
func (c *ComparisonReport) Get(name string) *ModeResult {
	for _, r := range c.Results {
		if r.ModeName == name {
			return r
		}
	}
	return nil
}

// PrintTable 输出 Markdown 对比表格
func (c *ComparisonReport) PrintTable(t *testing.T) {
	t.Helper()
	if len(c.Results) == 0 {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== Comparison: %s ===\n\n", c.Scenario))

	// 表头
	sb.WriteString("| Metric              |")
	for _, r := range c.Results {
		sb.WriteString(fmt.Sprintf(" %-12s |", r.ModeName))
	}
	sb.WriteString("\n|---------------------|")
	for range c.Results {
		sb.WriteString("--------------|")
	}

	// M1
	sb.WriteString("\n| M1 Correctness      |")
	for _, r := range c.Results {
		sb.WriteString(fmt.Sprintf(" %5.1f%%       |", r.Correctness))
	}

	// M5
	sb.WriteString("\n| M5 Preemption       |")
	for _, r := range c.Results {
		sb.WriteString(fmt.Sprintf(" %-12s |", boolStr(r.PreemptionOK)))
	}
	sb.WriteString("\n| M5 Recovery         |")
	for _, r := range c.Results {
		sb.WriteString(fmt.Sprintf(" %-12s |", boolStr(r.RecoveryOK)))
	}

	// BB checkpoints
	if len(c.Results[0].BBCheckResults) > 0 {
		for i, chk := range c.Results[0].BBCheckResults {
			sb.WriteString(fmt.Sprintf("\n| BB %-17s |", chk.Key))
			for _, r := range c.Results {
				if i < len(r.BBCheckResults) {
					sb.WriteString(fmt.Sprintf(" %-12s |", boolStr(r.BBCheckResults[i].Pass)))
				}
			}
		}
	}
	sb.WriteString("\n")
	t.Log(sb.String())
}

// PrintModeDetail 输出单模式详细日志
func PrintModeDetail(t *testing.T, mode *ModeResult) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n=== %s ===\n", mode.ModeName))
	prevState := ""
	for _, rec := range mode.Records {
		if rec.EventArrived != "" || rec.State != prevState {
			sb.WriteString(fmt.Sprintf("  Tick %3d: %-10s threat=%.1f", rec.Tick, rec.State, rec.ThreatLevel))
			if rec.EventArrived != "" {
				sb.WriteString(fmt.Sprintf("  [%s]", rec.EventArrived))
			}
			if rec.Transitioned {
				sb.WriteString(fmt.Sprintf("  (%s→%s)", prevState, rec.State))
			}
			sb.WriteString("\n")
		}
		prevState = rec.State
	}
	sb.WriteString(fmt.Sprintf("  correct=%d/%d (%.1f%%)\n", mode.CorrectChecks, mode.TotalChecks, mode.Correctness))
	t.Log(sb.String())
}

func boolStr(b bool) string {
	if b {
		return "PASS"
	}
	return "FAIL"
}
