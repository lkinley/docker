package daemon

import (
	"os"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/parsers/kernel"
	"github.com/docker/docker/pkg/parsers/operatingsystem"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

func (daemon *Daemon) CmdInfo(job *engine.Job) engine.Status {
	images, _ := daemon.Graph().Map()
	var imgcount int
	if images == nil {
		imgcount = 0
	} else {
		imgcount = len(images)
	}
	kernelVersion := "<unknown>"
	if kv, err := kernel.GetKernelVersion(); err == nil {
		kernelVersion = kv.String()
	}

	operatingSystem := "<unknown>"
	if s, err := operatingsystem.GetOperatingSystem(); err == nil {
		operatingSystem = s
	}
	if inContainer, err := operatingsystem.IsContainerized(); err != nil {
		log.Errorf("Could not determine if daemon is containerized: %v", err)
		operatingSystem += " (error determining if containerized)"
	} else if inContainer {
		operatingSystem += " (containerized)"
	}

	meminfo, err := system.ReadMemInfo()
	if err != nil {
		log.Errorf("Could not read system memory info: %v", err)
	}

	// if we still have the original dockerinit binary from before we copied it locally, let's return the path to that, since that's more intuitive (the copied path is trivial to derive by hand given VERSION)
	initPath := utils.DockerInitPath("")
	if initPath == "" {
		// if that fails, we'll just return the path from the daemon
		initPath = daemon.SystemInitPath()
	}

	cjob := job.Eng.Job("subscribers_count")
	env, _ := cjob.Stdout.AddEnv()
	if err := cjob.Run(); err != nil {
		return job.Error(err)
	}
	registryJob := job.Eng.Job("registry_config")
	registryEnv, _ := registryJob.Stdout.AddEnv()
	if err := registryJob.Run(); err != nil {
		return job.Error(err)
	}
	registryConfig := registry.ServiceConfig{}
	if err := registryEnv.GetJson("config", &registryConfig); err != nil {
		return job.Error(err)
	}
	v := &engine.Env{}
	v.SetJson("ID", daemon.ID)
	v.SetInt("Containers", len(daemon.List()))
	v.SetInt("Images", imgcount)
	v.Set("Driver", daemon.GraphDriver().String())
	v.SetJson("DriverStatus", daemon.GraphDriver().Status())
	v.SetBool("MemoryLimit", daemon.SystemConfig().MemoryLimit)
	v.SetBool("SwapLimit", daemon.SystemConfig().SwapLimit)
	v.SetBool("IPv4Forwarding", !daemon.SystemConfig().IPv4ForwardingDisabled)
	v.SetBool("Debug", os.Getenv("DEBUG") != "")
	v.SetInt("NFd", utils.GetTotalUsedFds())
	v.SetInt("NGoroutines", runtime.NumGoroutine())
	v.Set("ExecutionDriver", daemon.ExecutionDriver().Name())
	v.SetInt("NEventsListener", env.GetInt("count"))
	v.Set("KernelVersion", kernelVersion)
	v.Set("OperatingSystem", operatingSystem)
	v.Set("IndexServerAddress", registry.IndexServerAddress())
	v.SetJson("RegistryConfig", registryConfig)
	v.Set("InitSha1", dockerversion.INITSHA1)
	v.Set("InitPath", initPath)
	v.SetInt("NCPU", runtime.NumCPU())
	v.SetInt64("MemTotal", meminfo.MemTotal)
	v.Set("DockerRootDir", daemon.Config().Root)
	if hostname, err := os.Hostname(); err == nil {
		v.SetJson("Name", hostname)
	}
	v.SetList("Labels", daemon.Config().Labels)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return job.Error(err)
	}
	return engine.StatusOK
}
