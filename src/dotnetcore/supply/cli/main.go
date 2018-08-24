package main

import (

	// _ "dotnetcore/hooks"
	"dotnetcore/apt"
	"dotnetcore/config"
	"dotnetcore/project"
	"dotnetcore/supply"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cloudfoundry/libbuildpack"
)

func main() {
	logger := libbuildpack.NewLogger(os.Stdout)
	
	cmd := exec.Command("find", ".")
	cmd.Dir = "/tmp/cache"
	cmd.Stdout = os.Stdout
	cmd.Run()

	buildpackDir, err := libbuildpack.GetBuildpackDir()
	if err != nil {
		logger.Error("Unable to determine buildpack directory: %s", err.Error())
		os.Exit(9)
	}

	manifest, err := libbuildpack.NewManifest(buildpackDir, logger, time.Now())
	if err != nil {
		logger.Error("Unable to load buildpack manifest: %s", err.Error())
		os.Exit(10)
	}
	installer := libbuildpack.NewInstaller(manifest)

	stager := libbuildpack.NewStager(os.Args[1:], logger, manifest)
	if err := stager.CheckBuildpackValid(); err != nil {
		os.Exit(11)
	}

	if err = installer.SetAppCacheDir(stager.CacheDir()); err != nil {
		logger.Error("Unable to setup appcache: %s", err)
		os.Exit(18)
	}
	if err = manifest.ApplyOverride(stager.DepsDir()); err != nil {
		logger.Error("Unable to apply override.yml files: %s", err)
		os.Exit(17)
	}

	err = libbuildpack.RunBeforeCompile(stager)
	if err != nil {
		logger.Error("Before Compile: %s", err.Error())
		os.Exit(12)
	}

	if err := os.MkdirAll(filepath.Join(stager.DepDir(), "bin"), 0755); err != nil {
		logger.Error("Unable to create bin directory: %s", err.Error())
		os.Exit(13)
	}

	err = stager.SetStagingEnvironment()
	if err != nil {
		logger.Error("Unable to setup environment variables: %s", err.Error())
		os.Exit(14)
	}
	
	if exists, err := libbuildpack.FileExists(filepath.Join(stager.BuildDir(), "apt.yml")); err != nil {
		logger.Error("Unable to test existence of apt.yml: %s", err.Error())
		os.Exit(16)
	} else if !exists {
		logger.Error("Apt buildpack requires apt.yml\n(https://github.com/cloudfoundry/apt-buildpack/blob/master/fixtures/simple/apt.yml)")
		if exists, err := libbuildpack.FileExists(filepath.Join(stager.BuildDir(), "Aptfile")); err != nil || exists {
			logger.Error("Aptfile is deprecated. Please convert to apt.yml")
		}
		os.Exit(17)
	}
 	command := &libbuildpack.Command{}
	a := apt.New(command, filepath.Join(stager.BuildDir(), "apt.yml"), stager.CacheDir(), filepath.Join(stager.DepDir(), "apt"))
	if err := a.Setup(); err != nil {
		logger.Error("Unable to initialize apt package: %s", err.Error())
		os.Exit(13)
	}

	cfg := &config.Config{}

	s := supply.Supplier{
		Stager:    stager,
		Apt: a,
		Installer: installer,
		Manifest:  manifest,
		Log:       logger,
		Command:   &libbuildpack.Command{},
		Config:    cfg,
		Project:   project.New(stager.BuildDir(), stager.DepDir(), stager.DepsIdx()),
	}

	err = supply.Run(&s)
	if err != nil {
		os.Exit(15)
	}

	if err := stager.WriteConfigYml(cfg); err != nil {
		logger.Error("Error writing config.yml: %s", err.Error())
		os.Exit(16)
	}
	if err = installer.CleanupAppCache(); err != nil {
		logger.Error("Unable to clean up app cache: %s", err)
		os.Exit(19)
	}
}
