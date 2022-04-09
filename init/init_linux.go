// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/u-root/u-root/pkg/cmdline"
	"github.com/u-root/u-root/pkg/uflag"
	"github.com/u-root/u-root/pkg/ulog"
)

func quiet() {
	if !*verbose {
		// Only messages more severe than "notice" are printed.
		if err := ulog.KernelLog.SetConsoleLogLevel(ulog.KLogNotice); err != nil {
			log.Printf("Could not set log level: %v", err)
		}
	}
}

func osInitGo() *initCmds {
	// Backwards compatibility for the transition from uroot.nohwrng to
	// UROOT_NOHWRNG=1 on kernel commandline.
	if cmdline.ContainsFlag("uroot.nohwrng") {
		os.Setenv("UROOT_NOHWRNG", "1")
		log.Printf("Deprecation warning: use UROOT_NOHWRNG=1 on kernel cmdline instead of uroot.nohwrng")
	}

	// Turn off job control when test mode is on.
	ctty := WithTTYControl(false) // !*test)

	// Install modules before exec-ing into user mode below
	// the install all modules is just done wrong. Ignore for now.
	// fix later.
	// InstallAllModules()

	// systemd is "special". If we are supposed to run systemd, we're
	// going to exec, and if we're going to exec, we're done here.
	// systemd uber alles.
	initFlags := cmdline.GetInitFlagMap()

	// systemd gets upset when it discovers it isn't really process 1, so
	// we can't start it in its own namespace. I just love systemd.
	systemd, present := initFlags["systemd"]
	systemdEnabled, boolErr := strconv.ParseBool(systemd)
	if present && boolErr == nil && systemdEnabled {
		if err := syscall.Exec("/inito", []string{"/inito"}, os.Environ()); err != nil {
			log.Printf("Lucky you, systemd failed: %v", err)
		}
	}

	// Allows passing args to uinit via kernel parameters, for example:
	//
	// uroot.uinitargs="-v --foobar"
	//
	// We also allow passing args to uinit via a flags file in
	// /etc/uinit.flags.
	args := cmdline.GetUinitArgs()
	if contents, err := os.ReadFile("/etc/uinit.flags"); err == nil {
		args = append(args, uflag.FileToArgv(string(contents))...)
	}
	uinitArgs := WithArguments(args...)

	return &initCmds{
		cmds: []*exec.Cmd{
			// inito is (optionally) created by the u-root command when the
			// u-root initramfs is merged with an existing initramfs that
			// has a /init. The name inito means "original /init" There may
			// be an inito if we are building on an existing initramfs. All
			// initos need their own pid space.
			Command(filepath.Join(bin, "elvish")),
			Command("elvish"),
			Command("inito", WithCloneFlags(syscall.CLONE_NEWPID), ctty),
			Command("uinit", ctty, uinitArgs),
			Command("defaultsh", ctty),
			Command("sh", ctty),
		},
	}
}
