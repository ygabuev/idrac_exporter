package collector

import (
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/mrlhansen/idrac_exporter/internal/config"
	"github.com/mrlhansen/idrac_exporter/internal/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

var mu sync.Mutex
var collectors = map[string]*Collector{}

type Collector struct {
	// Internal variables
	client     *Client
	registry   *prometheus.Registry
	collected  *sync.Cond
	collecting bool
	retries    uint
	errors     uint
	builder    *strings.Builder

	// Exporter
	ExporterBuildInfo         *prometheus.Desc
	ExporterScrapeErrorsTotal *prometheus.Desc

	// System
	SystemPowerOn      *prometheus.Desc
	SystemHealth       *prometheus.Desc
	SystemIndicatorLED *prometheus.Desc
	SystemMemorySize   *prometheus.Desc
	SystemCpuCount     *prometheus.Desc
	SystemBiosInfo     *prometheus.Desc
	SystemMachineInfo  *prometheus.Desc

	// Sensors
	SensorsTemperature *prometheus.Desc
	SensorsFanSpeed    *prometheus.Desc

	// Power supply
	PowerSupplyOutputWatts       *prometheus.Desc
	PowerSupplyInputWatts        *prometheus.Desc
	PowerSupplyCapacityWatts     *prometheus.Desc
	PowerSupplyInputVoltage      *prometheus.Desc
	PowerSupplyEfficiencyPercent *prometheus.Desc

	// Power control
	PowerControlConsumedWatts    *prometheus.Desc
	PowerControlCapacityWatts    *prometheus.Desc
	PowerControlMinConsumedWatts *prometheus.Desc
	PowerControlMaxConsumedWatts *prometheus.Desc
	PowerControlAvgConsumedWatts *prometheus.Desc
	PowerControlInterval         *prometheus.Desc

	// System event log
	SelEntry *prometheus.Desc

	// Disk drives
	DriveInfo     *prometheus.Desc
	DriveHealth   *prometheus.Desc
	DriveCapacity *prometheus.Desc

	// Memory modules
	MemoryModuleInfo     *prometheus.Desc
	MemoryModuleHealth   *prometheus.Desc
	MemoryModuleCapacity *prometheus.Desc
	MemoryModuleSpeed    *prometheus.Desc
}

func NewCollector() *Collector {
	prefix := config.Config.MetricsPrefix

	collector := &Collector{
		ExporterBuildInfo: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "exporter", "build_info"),
			"Constant metric with build information for the exporter",
			nil, prometheus.Labels{
				"version":   version.Version,
				"revision":  version.Revision,
				"goversion": runtime.Version(),
			},
		),
		ExporterScrapeErrorsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "exporter", "scrape_errors_total"),
			"Total number of errors encountered while scraping target",
			nil, nil,
		),
		SystemPowerOn: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "system", "power_on"),
			"Power state of the system",
			nil, nil,
		),
		SystemHealth: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "system", "health"),
			"Health status of the system",
			[]string{"status"}, nil,
		),
		SystemIndicatorLED: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "system", "indicator_led_on"),
			"Indicator LED state of the system",
			[]string{"state"}, nil,
		),
		SystemMemorySize: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "system", "memory_size_bytes"),
			"Total memory size of the system in bytes",
			nil, nil,
		),
		SystemCpuCount: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "system", "cpu_count"),
			"Total number of CPUs in the system",
			[]string{"model"}, nil,
		),
		SystemBiosInfo: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "system", "bios_info"),
			"Information about the BIOS",
			[]string{"version"}, nil,
		),
		SystemMachineInfo: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "system", "machine_info"),
			"Information about the machine",
			[]string{"manufacturer", "model", "serial", "sku"}, nil,
		),
		SensorsTemperature: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "sensors", "temperature"),
			"Sensors reporting temperature measurements",
			[]string{"id", "name", "units"}, nil,
		),
		SensorsFanSpeed: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "sensors", "fan_speed"),
			"Sensors reporting fan speed measurements",
			[]string{"id", "name", "units"}, nil,
		),
		PowerSupplyOutputWatts: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_supply", "output_watts"),
			"Power supply output in watts",
			[]string{"id"}, nil,
		),
		PowerSupplyInputWatts: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_supply", "input_watts"),
			"Power supply input in watts",
			[]string{"id"}, nil,
		),
		PowerSupplyCapacityWatts: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_supply", "capacity_watts"),
			"Power supply capacity in watts",
			[]string{"id"}, nil,
		),
		PowerSupplyInputVoltage: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_supply", "input_voltage"),
			"Power supply input voltage",
			[]string{"id"}, nil,
		),
		PowerSupplyEfficiencyPercent: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_supply", "efficiency_percent"),
			"Power supply efficiency in percentage",
			[]string{"id"}, nil,
		),
		PowerControlConsumedWatts: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_control", "consumed_watts"),
			"Consumption of power control system in watts",
			[]string{"id", "name"}, nil,
		),
		PowerControlCapacityWatts: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_control", "capacity_watts"),
			"Capacity of power control system in watts",
			[]string{"id", "name"}, nil,
		),
		PowerControlMinConsumedWatts: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_control", "min_consumed_watts"),
			"Minimum consumption of power control system during the reported interval",
			[]string{"id", "name"}, nil,
		),
		PowerControlMaxConsumedWatts: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_control", "max_consumed_watts"),
			"Maximum consumption of power control system during the reported interval",
			[]string{"id", "name"}, nil,
		),
		PowerControlAvgConsumedWatts: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_control", "avg_consumed_watts"),
			"Average consumption of power control system during the reported interval",
			[]string{"id", "name"}, nil,
		),
		PowerControlInterval: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "power_control", "interval_in_minutes"),
			"Interval for measurements of power control system",
			[]string{"id", "name"}, nil,
		),
		SelEntry: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "sel", "entry"),
			"Entry from the system event log",
			[]string{"id", "message", "component", "severity"}, nil,
		),
		DriveInfo: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "drive", "info"),
			"Information about disk drives",
			[]string{"id", "manufacturer", "mediatype", "model", "name", "protocol", "serial", "slot"}, nil,
		),
		DriveHealth: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "drive", "health"),
			"Health status for disk drives",
			[]string{"id", "status"}, nil,
		),
		DriveCapacity: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "drive", "capacity_bytes"),
			"Capacity of disk drives in bytes",
			[]string{"id"}, nil,
		),
		MemoryModuleInfo: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "memory_module", "info"),
			"Information about memory modules",
			[]string{"id", "ecc", "manufacturer", "type", "name", "serial", "rank"}, nil,
		),
		MemoryModuleHealth: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "memory_module", "health"),
			"Health status for memory modules",
			[]string{"id", "status"}, nil,
		),
		MemoryModuleCapacity: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "memory_module", "capacity_bytes"),
			"Capacity of memory modules in bytes",
			[]string{"id"}, nil,
		),
		MemoryModuleSpeed: prometheus.NewDesc(
			prometheus.BuildFQName(prefix, "memory_module", "speed_mhz"),
			"Speed of memory modules in Mhz",
			[]string{"id"}, nil,
		),
	}

	collector.builder = new(strings.Builder)
	collector.collected = sync.NewCond(new(sync.Mutex))
	collector.registry = prometheus.NewRegistry()
	collector.registry.Register(collector)

	return collector
}

func (collector *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.ExporterBuildInfo
	ch <- collector.ExporterScrapeErrorsTotal
	ch <- collector.SystemPowerOn
	ch <- collector.SystemHealth
	ch <- collector.SystemIndicatorLED
	ch <- collector.SystemMemorySize
	ch <- collector.SystemCpuCount
	ch <- collector.SystemBiosInfo
	ch <- collector.SystemMachineInfo
	ch <- collector.SensorsTemperature
	ch <- collector.SensorsFanSpeed
	ch <- collector.PowerSupplyOutputWatts
	ch <- collector.PowerSupplyInputWatts
	ch <- collector.PowerSupplyCapacityWatts
	ch <- collector.PowerSupplyInputVoltage
	ch <- collector.PowerSupplyEfficiencyPercent
	ch <- collector.PowerControlConsumedWatts
	ch <- collector.PowerControlCapacityWatts
	ch <- collector.PowerControlMinConsumedWatts
	ch <- collector.PowerControlMaxConsumedWatts
	ch <- collector.PowerControlAvgConsumedWatts
	ch <- collector.PowerControlInterval
	ch <- collector.SelEntry
	ch <- collector.DriveInfo
	ch <- collector.DriveHealth
	ch <- collector.DriveCapacity
	ch <- collector.MemoryModuleInfo
	ch <- collector.MemoryModuleHealth
	ch <- collector.MemoryModuleCapacity
	ch <- collector.MemoryModuleSpeed
}

func (collector *Collector) Collect(ch chan<- prometheus.Metric) {
	if config.Config.Collect.System {
		err := collector.client.RefreshSystem(collector, ch)
		if err != nil {
			collector.errors++
		}
	}
	if config.Config.Collect.Sensors {
		err := collector.client.RefreshSensors(collector, ch)
		if err != nil {
			collector.errors++
		}
	}
	if config.Config.Collect.Power {
		err := collector.client.RefreshPower(collector, ch)
		if err != nil {
			collector.errors++
		}
	}
	if config.Config.Collect.SEL {
		err := collector.client.RefreshIdracSel(collector, ch)
		if err != nil {
			collector.errors++
		}
	}
	if config.Config.Collect.Storage {
		err := collector.client.RefreshStorage(collector, ch)
		if err != nil {
			collector.errors++
		}
	}
	if config.Config.Collect.Memory {
		err := collector.client.RefreshMemory(collector, ch)
		if err != nil {
			collector.errors++
		}
	}
	ch <- prometheus.MustNewConstMetric(collector.ExporterBuildInfo, prometheus.UntypedValue, 1)
	ch <- prometheus.MustNewConstMetric(collector.ExporterScrapeErrorsTotal, prometheus.GaugeValue, float64(collector.errors))
}

func (collector *Collector) Gather() (string, error) {
	collector.collected.L.Lock()

	// If a collection is already in progress wait for it to complete and return the cached data
	if collector.collecting {
		collector.collected.Wait()
		metrics := collector.builder.String()
		collector.collected.L.Unlock()
		return metrics, nil
	}

	// Set collecting to true and let other goroutines enter in critical section
	collector.collecting = true
	collector.collected.L.Unlock()

	// Defer set collecting to false and wake waiting goroutines
	defer func() {
		collector.collected.L.Lock()
		collector.collected.Broadcast()
		collector.collecting = false
		collector.collected.L.Unlock()
	}()

	// Collect metrics
	collector.builder.Reset()

	m, err := collector.registry.Gather()
	if err != nil {
		return "", err
	}

	for i := range m {
		expfmt.MetricFamilyToText(collector.builder, m[i])
	}

	return collector.builder.String(), nil
}

// Resets an existing collector of the given target
func Reset(target string) {
	mu.Lock()
	_, ok := collectors[target]
	if ok {
		delete(collectors, target)
	}
	mu.Unlock()
}

func GetCollector(target string) (*Collector, error) {
	mu.Lock()
	collector, ok := collectors[target]
	if !ok {
		collector = NewCollector()
		collectors[target] = collector
	}
	mu.Unlock()

	// Do not act concurrently on the same host
	collector.collected.L.Lock()
	defer collector.collected.L.Unlock()

	// Try to instantiate a new Redfish host
	if collector.client == nil {
		if collector.retries < config.Config.Retries {
			host := config.Config.GetHostCfg(target)
			c, err := NewClient(host)
			if err != nil {
				collector.retries++
				return nil, err
			} else {
				collector.client = c
			}
		} else {
			return nil, fmt.Errorf("host unreachable after %d retries", collector.retries)
		}
	}

	return collector, nil
}
