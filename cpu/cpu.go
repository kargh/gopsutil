package cpu

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kargh/gopsutil/v3/internal/common"
)

// TimesStat contains the amounts of time the CPU has spent performing different
// kinds of work. Time units are in seconds. It is based on linux /proc/stat file.
type TimesStat struct {
	CPU       string  `json:"cpu"`
	User      float64 `json:"user"`
	System    float64 `json:"system"`
	Idle      float64 `json:"idle"`
	Nice      float64 `json:"nice"`
	Iowait    float64 `json:"iowait"`
	Irq       float64 `json:"irq"`
	Softirq   float64 `json:"softirq"`
	Steal     float64 `json:"steal"`
	Guest     float64 `json:"guest"`
	GuestNice float64 `json:"guestNice"`
}

type InfoStat struct {
	CPU        int32    `json:"cpu"`
	VendorID   string   `json:"vendorId"`
	Family     string   `json:"family"`
	Model      string   `json:"model"`
	Stepping   int32    `json:"stepping"`
	PhysicalID string   `json:"physicalId"`
	CoreID     string   `json:"coreId"`
	Cores      int32    `json:"cores"`
	ModelName  string   `json:"modelName"`
	Mhz        float64  `json:"mhz"`
	CacheSize  int32    `json:"cacheSize"`
	Flags      []string `json:"flags"`
	Microcode  string   `json:"microcode"`
}

type lastPercent struct {
	sync.Mutex
	lastCPUTimes    []TimesStat
	lastPerCPUTimes []TimesStat
}

var (
	lastCPUPercent lastPercent
	invoke         common.Invoker = common.Invoke{}
)

func init() {
	lastCPUPercent.Lock()
	lastCPUPercent.lastCPUTimes, _ = Times(false)
	lastCPUPercent.lastPerCPUTimes, _ = Times(true)
	lastCPUPercent.Unlock()
}

// Counts returns the number of physical or logical cores in the system
func Counts(logical bool) (int, error) {
	return CountsWithContext(context.Background(), logical)
}

func (c TimesStat) String() string {
	v := []string{
		`"cpu":"` + c.CPU + `"`,
		`"user":` + strconv.FormatFloat(c.User, 'f', 1, 64),
		`"system":` + strconv.FormatFloat(c.System, 'f', 1, 64),
		`"idle":` + strconv.FormatFloat(c.Idle, 'f', 1, 64),
		`"nice":` + strconv.FormatFloat(c.Nice, 'f', 1, 64),
		`"iowait":` + strconv.FormatFloat(c.Iowait, 'f', 1, 64),
		`"irq":` + strconv.FormatFloat(c.Irq, 'f', 1, 64),
		`"softirq":` + strconv.FormatFloat(c.Softirq, 'f', 1, 64),
		`"steal":` + strconv.FormatFloat(c.Steal, 'f', 1, 64),
		`"guest":` + strconv.FormatFloat(c.Guest, 'f', 1, 64),
		`"guestNice":` + strconv.FormatFloat(c.GuestNice, 'f', 1, 64),
	}

	return `{` + strings.Join(v, ",") + `}`
}

// Deprecated: Total returns the total number of seconds in a CPUTimesStat
// Please do not use this internal function.
func (c TimesStat) Total() float64 {
	total := c.User + c.System + c.Idle + c.Nice + c.Iowait + c.Irq +
		c.Softirq + c.Steal + c.Guest + c.GuestNice

	return total
}

func UsedTime(t1, t2 TimesStat) (TimesStat) {
	var usedTime TimesStat

	usedTime.User = t2.User - t1.User
	usedTime.System = t2.System - t1.System
	usedTime.Idle = t2.Idle - t1.Idle
	usedTime.Nice = t2.Nice - t1.Nice
	usedTime.Iowait = t2.Iowait - t1.Iowait
	usedTime.Irq = t2.Irq - t1.Irq
	usedTime.Softirq = t2.Softirq - t1.Softirq
	usedTime.Steal = t2.Steal - t1.Steal
	usedTime.Guest = t2.Guest - t1.Guest
	usedTime.GuestNice = t2.GuestNice - t1.GuestNice

	tot := usedTime.Total()
        if runtime.GOOS == "linux" {
                tot -= usedTime.Guest     // Linux 2.6.24+
                tot -= usedTime.GuestNice // Linux 3.2.0+
        }

        //busy := tot - usedTime.Idle - usedTime.Iowait

	usedTime.User = math.Min(100, math.Max(0, usedTime.User/tot*100))
	usedTime.System = math.Min(100, math.Max(0, usedTime.System/tot*100))
	usedTime.Idle = math.Min(100, math.Max(0, usedTime.Idle/tot*100))
	usedTime.Nice = math.Min(100, math.Max(0, usedTime.Nice/tot*100))
	usedTime.Iowait = math.Min(100, math.Max(0, usedTime.Iowait/tot*100))
	usedTime.Irq = math.Min(100, math.Max(0, usedTime.Irq/tot*100))
	usedTime.Softirq = math.Min(100, math.Max(0, usedTime.Softirq/tot*100))
	usedTime.Steal = math.Min(100, math.Max(0, usedTime.Steal/tot*100))
	usedTime.Guest = math.Min(100, math.Max(0, usedTime.Guest/tot*100))
	usedTime.GuestNice = math.Min(100, math.Max(0, usedTime.GuestNice/tot*100))

	return usedTime
}

func (c InfoStat) String() string {
	s, _ := json.Marshal(c)
	return string(s)
}

func getAllBusy(t TimesStat) (float64, float64) {
	tot := t.Total()
	if runtime.GOOS == "linux" {
		tot -= t.Guest     // Linux 2.6.24+
		tot -= t.GuestNice // Linux 3.2.0+
	}

	busy := tot - t.Idle - t.Iowait

	return tot, busy
}

func calculateBusy(t1, t2 TimesStat) float64 {
	t1All, t1Busy := getAllBusy(t1)
	t2All, t2Busy := getAllBusy(t2)

	if t2Busy <= t1Busy {
		return 0
	}
	if t2All <= t1All {
		return 100
	}
	return math.Min(100, math.Max(0, (t2Busy-t1Busy)/(t2All-t1All)*100))
}

func calculateAllBusy(t1, t2 []TimesStat) ([]float64, error) {
	// Make sure the CPU measurements have the same length.
	if len(t1) != len(t2) {
		return nil, fmt.Errorf(
			"received two CPU counts: %d != %d",
			len(t1), len(t2),
		)
	}

	ret := make([]float64, len(t1))
	for i, t := range t2 {
		ret[i] = calculateBusy(t1[i], t)
	}
	return ret, nil
}

func calculateAllBusyMode(t1, t2 []TimesStat) (TimesStat, error) {
	// Make sure the CPU measurements have the same length.
	//if len(t1) != len(t2) {
	//	return _, fmt.Errorf(
	//		"received two CPU counts: %d != %d",
	//		len(t1), len(t2),
	//	)
	//}

	//ret := make([]TimesStat, len(t1))
	var ret TimesStat
	for i, t := range t2 {
		ret = UsedTime(t1[i], t)
	}
	return ret, nil
}

// Percent calculates the percentage of cpu used either per CPU or combined.
// If an interval of 0 is given it will compare the current cpu times against the last call.
// Returns one value per cpu, or a single value if percpu is set to false.
func Percent(interval time.Duration, percpu bool) ([]float64, error) {
	return PercentWithContext(context.Background(), interval, percpu)
}

func PercentCpuTimes(interval time.Duration, percpu bool) (TimesStat, error) {
	return PercentWithContextMode(context.Background(), interval, percpu)
}

func PercentCpuTimes2(interval time.Duration, percpu bool) (TimesStat, error) {
	return PercentWithContextMode(context.Background(), interval, percpu)
}

func PercentWithContext(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
	if interval <= 0 {
		return percentUsedFromLastCallWithContext(ctx, percpu)
	}

	// Get CPU usage at the start of the interval.
	cpuTimes1, err := TimesWithContext(ctx, percpu)
	if err != nil {
		return nil, err
	}

	if err := common.Sleep(ctx, interval); err != nil {
		return nil, err
	}

	// And at the end of the interval.
	cpuTimes2, err := TimesWithContext(ctx, percpu)
	if err != nil {
		return nil, err
	}

	return calculateAllBusy(cpuTimes1, cpuTimes2)
}

func PercentWithContextMode(ctx context.Context, interval time.Duration, percpu bool) (TimesStat, error) {
	return percentUsedFromLastCallWithContextMode(ctx, percpu)
}

func percentUsedFromLastCall(percpu bool) ([]float64, error) {
	return percentUsedFromLastCallWithContext(context.Background(), percpu)
}

func percentUsedFromLastCallWithContext(ctx context.Context, percpu bool) ([]float64, error) {
	cpuTimes, err := TimesWithContext(ctx, percpu)
	if err != nil {
		return nil, err
	}
	lastCPUPercent.Lock()
	defer lastCPUPercent.Unlock()
	var lastTimes []TimesStat
	if percpu {
		lastTimes = lastCPUPercent.lastPerCPUTimes
		lastCPUPercent.lastPerCPUTimes = cpuTimes
	} else {
		lastTimes = lastCPUPercent.lastCPUTimes
		lastCPUPercent.lastCPUTimes = cpuTimes
	}

	if lastTimes == nil {
		return nil, fmt.Errorf("error getting times for cpu percent. lastTimes was nil")
	}
	return calculateAllBusy(lastTimes, cpuTimes)
}

func percentUsedFromLastCallWithContextMode(ctx context.Context, percpu bool) (TimesStat, error) {
	cpuTimes, _ := TimesWithContext(ctx, percpu)
	//if err != nil {
	//	return _, err
	//}
	lastCPUPercent.Lock()
	defer lastCPUPercent.Unlock()
	var lastTimes []TimesStat
	if percpu {
		lastTimes = lastCPUPercent.lastPerCPUTimes
		lastCPUPercent.lastPerCPUTimes = cpuTimes
	} else {
		lastTimes = lastCPUPercent.lastCPUTimes
		lastCPUPercent.lastCPUTimes = cpuTimes
	}

	//if lastTimes == nil {
	//	return _, fmt.Errorf("error getting times for cpu percent. lastTimes was nil")
	//}
	return calculateAllBusyMode(lastTimes, cpuTimes)
}
