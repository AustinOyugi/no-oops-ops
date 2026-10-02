package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/tui"
	"github.com/AustinOyugi/no-oops-ops/internal/upgrade"
)

type paletteSpec struct {
	label, description string
	command            []string
	required           []string
	flags              []string
	fields             []tui.Field
}

func uiPalette(cfg config.Config, row *tui.Row) ([]tui.Command, error) {
	apps, err := catalog.Load(cfg.Workspace)
	if err != nil {
		return nil, err
	}
	names := []string{}
	services := map[string][]string{}
	for name := range apps.Apps {
		names = append(names, name)
		path, err := catalog.Resolve(cfg.Workspace, name)
		if err == nil {
			services[name], _ = manifest.Services(path)
		}
	}
	sort.Strings(names)
	defaults := map[string]string{"environment": "", "app": "", "service": ""}
	if row != nil && !row.Untracked && row.Environment != "platform" {
		defaults["environment"] = row.Environment
		if actions, err := uiActions(cfg, *row); err == nil {
			for _, action := range actions {
				if action.Label == "Release" {
					defaults["app"] = action.Args[4]
					defaults["service"] = action.Args[6]
				}
			}
		}
	}
	text := func(key, label, value string) tui.Field { return tui.Field{Key: key, Label: label, Default: value} }
	boolean := func(key, label string) tui.Field {
		return tui.Field{Key: key, Label: label, Default: "false", Boolean: true}
	}
	targetFields := []tui.Field{text("environment", "Environment", defaults["environment"]), {Key: "app", Label: "App", Default: defaults["app"], Options: func(map[string]string) []string { return names }}, boolean("all", "All services"), {Key: "service", Label: "Service", Default: defaults["service"], Options: func(v map[string]string) []string { return services[v["app"]] }, Disabled: func(v map[string]string) bool { return v["all"] == "true" }}}
	specs := []paletteSpec{
		{label: "Version", description: "Print the CLI version", command: []string{"version"}},
		{label: "Initialize workspace", description: "Create a new workspace", command: []string{"init"}, required: []string{"path"}, fields: []tui.Field{text("path", "New workspace path", "")}},
		{label: "Install", description: "Install or reconcile platform services", command: []string{"install"}},
		{label: "Uninstall", description: "Remove platform and managed app stacks; purge also deletes persistent registry data", command: []string{"uninstall"}, flags: []string{"purge"}, fields: []tui.Field{boolean("purge", "Purge persistent data")}},
		{label: "Status", description: "Inspect recorded platform status", command: []string{"status"}},
		{label: "Doctor", description: "Check platform prerequisites", command: []string{"doctor"}, flags: []string{"deploy-ready"}, fields: []tui.Field{boolean("deploy-ready", "Deployment prerequisites only")}},
		{label: "Upgrade check", description: "Check the latest available release", command: []string{"upgrade", "--check"}},
		{label: "Upgrade", description: "Switch CLI and catalog version; restart the dashboard afterward", command: []string{"upgrade"}, flags: []string{"dry-run", "allow-downgrade", "yes"}, fields: []tui.Field{text("to", "Target version or latest", "latest"), boolean("dry-run", "Dry run"), boolean("allow-downgrade", "Allow downgrade"), boolean("yes", "Skip CLI confirmation")}},
		{label: "Cleanup", description: "Preview retention cleanup; Apply enables deletion", command: []string{"cleanup"}, flags: []string{"apply", "orphaned"}, fields: []tui.Field{text("keep", "Records to retain", "3"), boolean("apply", "Apply deletions"), boolean("orphaned", "Include orphaned history")}},
		{label: "Secret set", description: "Value is requested in the CLI hidden prompt", command: []string{"secret", "set"}, required: []string{"environment", "key"}, fields: []tui.Field{text("environment", "Environment", defaults["environment"]), text("key", "Secret key", "")}},
		{label: "Secret delete", description: "Delete all versions of a secret key", command: []string{"secret", "delete"}, required: []string{"environment", "key"}, fields: []tui.Field{text("environment", "Environment", defaults["environment"]), text("key", "Secret key", "")}},
		{label: "Secret list", description: "List secret metadata without values", command: []string{"secret", "list"}, required: []string{"environment"}, fields: []tui.Field{text("environment", "Environment", defaults["environment"])}},
		{label: "Certificate import", description: "Import a certificate and private key from local files", command: []string{"certificate", "import"}, required: []string{"name", "certificate", "private-key"}, fields: []tui.Field{text("name", "Certificate name", ""), text("certificate", "Certificate path", ""), text("private-key", "Private key path", "")}},
	}
	for _, item := range []struct {
		label   string
		command []string
		flag    string
	}{{"Release", []string{"release"}, "deploy"}, {"Release list", []string{"release", "list"}, ""}, {"Deploy", []string{"deploy"}, "quick"}, {"Rollback", []string{"rollback"}, ""}, {"Remove", []string{"remove"}, ""}} {
		fields := append([]tui.Field{}, targetFields...)
		flags := []string{}
		if item.flag != "" {
			flags = append(flags, item.flag)
			label := "Quick rollout"
			if item.flag == "deploy" {
				label = "Deploy after release"
			}
			fields = append(fields, boolean(item.flag, label))
		}
		specs = append(specs, paletteSpec{label: item.label, description: "Run " + strings.Join(item.command, " ") + " for the selected app target", command: item.command, required: []string{"environment", "app"}, fields: fields, flags: flags})
	}
	var commands []tui.Command
	for _, spec := range specs {
		spec := spec
		commands = append(commands, tui.Command{Label: spec.label, Description: spec.description, Fields: spec.fields, Build: func(values map[string]string) (tui.Action, error) {
			args := []string{"--workspace", cfg.Workspace}
			args = append(args, spec.command...)
			for _, key := range spec.required {
				value := strings.TrimSpace(values[key])
				if value == "" {
					return tui.Action{}, fmt.Errorf("%s is required", key)
				}
				if strings.HasPrefix(value, "-") {
					return tui.Action{}, fmt.Errorf("%s cannot begin with a dash", key)
				}
				args = append(args, value)
			}
			if len(spec.required) > 1 && spec.required[1] == "app" {
				if _, ok := apps.Apps[values["app"]]; !ok {
					return tui.Action{}, fmt.Errorf("select a catalog app")
				}
				if values["all"] == "true" {
					args = append(args, "--all")
				} else {
					valid := false
					for _, service := range services[values["app"]] {
						if service == values["service"] {
							valid = true
						}
					}
					if !valid {
						return tui.Action{}, fmt.Errorf("select a service or All services")
					}
					args = append(args, "--service", values["service"])
				}
			}
			if spec.label == "Upgrade" {
				target := strings.TrimSpace(values["to"])
				if target != "latest" {
					if err := upgrade.ValidateReleaseTag(target); err != nil {
						return tui.Action{}, err
					}
				}
				if values["yes"] == "true" && target == "latest" {
					return tui.Action{}, fmt.Errorf("skip confirmation requires an exact version")
				}
				args = append(args, "--to", target)
			}
			if spec.label == "Cleanup" {
				keep, err := strconv.Atoi(values["keep"])
				if err != nil || keep < 1 {
					return tui.Action{}, fmt.Errorf("retain count must be a positive integer")
				}
				args = append(args, "--keep", strconv.Itoa(keep))
			}
			for _, flag := range spec.flags {
				if values[flag] == "true" {
					args = append(args, "--"+flag)
				}
			}
			return tui.Action{Label: spec.label, Args: args}, nil
		}})
	}
	return commands, nil
}
