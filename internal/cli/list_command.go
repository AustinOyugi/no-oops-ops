package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/tui"
	"github.com/spf13/cobra"
)

func newListCommand(ctx context.Context, rt *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List managed services in an environment",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := rt.configuration()
			if err != nil {
				return err
			}
			rows, err := tui.Services(ctx, cfg)
			if err != nil {
				return err
			}
			return printServices(cmd.OutOrStdout(), managedEnvironmentServices(rows, rt.selectedEnvironment))
		},
	}
}

func newPSCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "ps [service]",
		Short: "List running tasks for managed services in an environment",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := rt.configuration()
			if err != nil {
				return err
			}
			services, err := tui.Services(ctx, cfg)
			if err != nil {
				return err
			}
			services = managedEnvironmentServices(services, rt.selectedEnvironment)
			if len(args) == 1 {
				services, err = selectService(services, args[0])
				if err != nil {
					return err
				}
			}
			return printTasks(ctx, cmd.OutOrStdout(), services, all)
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Include stopped and historical tasks")
	return cmd
}

func managedEnvironmentServices(rows []tui.Row, environment string) []tui.Row {
	filtered := make([]tui.Row, 0, len(rows))
	for _, row := range rows {
		if !row.Untracked && row.Environment == environment {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func printServices(output io.Writer, services []tui.Row) error {
	if len(services) == 0 {
		_, err := fmt.Fprintln(output, "No managed services found.")
		return err
	}
	w := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "APP\tSERVICE\tREPLICAS\tSTATE"); err != nil {
		return err
	}
	for _, service := range services {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", service.App, service.Service, service.Replicas, service.State); err != nil {
			return err
		}
	}
	return w.Flush()
}

func printTasks(ctx context.Context, output io.Writer, services []tui.Row, all bool) error {
	if len(services) == 0 {
		_, err := fmt.Fprintln(output, "No managed services found.")
		return err
	}
	w := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "SERVICE\tTASK\tNODE\tSTATE\tAGE\tERROR"); err != nil {
		return err
	}
	count := 0
	for _, service := range services {
		tasks, err := tui.Tasks(ctx, service.Service)
		if err != nil {
			return err
		}
		for _, task := range tasks {
			if !all && task.State != "running" {
				continue
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", service.Service, task.ID, task.Node, task.State, tui.TaskAge(task, time.Now()), task.Error); err != nil {
				return err
			}
			count++
		}
	}
	if count == 0 {
		message := "No running tasks found."
		if all {
			message = "No tasks found."
		}
		if _, err := fmt.Fprintln(w, message); err != nil {
			return err
		}
	}
	return w.Flush()
}

func selectService(services []tui.Row, requested string) ([]tui.Row, error) {
	var exact, suffix []tui.Row
	for _, service := range services {
		if service.Service == requested {
			exact = append(exact, service)
			continue
		}
		if strings.HasSuffix(service.Service, "_"+requested) {
			suffix = append(suffix, service)
		}
	}
	if len(exact) == 1 {
		return exact, nil
	}
	if len(exact) > 1 {
		return nil, fmt.Errorf("service %q is ambiguous", requested)
	}
	if len(suffix) == 1 {
		return suffix, nil
	}
	if len(suffix) > 1 {
		return nil, fmt.Errorf("service %q matches multiple services; use the full service name", requested)
	}
	return nil, fmt.Errorf("service %q not found in the selected environment", requested)
}
