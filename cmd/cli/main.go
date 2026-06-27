// Kolibri Mesh CLI
// Command line tool for managing the mesh network
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	coordinatorURL string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mesh",
		Short: "Kolibri Mesh - Mesh VPN management",
		Long:  "Kolibri Mesh is a mesh VPN with AWG protocol for Kolibri infrastructure",
	}

	rootCmd.PersistentFlags().StringVar(&coordinatorURL, "coordinator", "http://localhost:8080", "Coordinator URL")

	// Add commands
	rootCmd.AddCommand(
		newStatusCmd(),
		newNodesCmd(),
		newAddCmd(),
		newRemoveCmd(),
		newConfigCmd(),
		newStartCmd(),
		newStopCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newStatusCmd creates status command
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show mesh status",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get(coordinatorURL + "/api/status")
			if err != nil {
				return fmt.Errorf("failed to get status: %w", err)
			}
			defer resp.Body.Close()

			var status map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
				return fmt.Errorf("failed to decode status: %w", err)
			}

			fmt.Println("Kolibri Mesh Status:")
			fmt.Printf("Coordinator: %s\n", coordinatorURL)
			fmt.Printf("Nodes: %v\n", len(status["nodes"].(map[string]interface{})))
			return nil
		},
	}
}

// newNodesCmd creates nodes command
func newNodesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "nodes",
		Short: "List all nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get(coordinatorURL + "/api/nodes")
			if err != nil {
				return fmt.Errorf("failed to get nodes: %w", err)
			}
			defer resp.Body.Close()

			var nodes []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
				return fmt.Errorf("failed to decode nodes: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tIP\tROLE\tSTATUS")
			for _, node := range nodes {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					node["id"], node["name"], node["ip"], node["role"], node["status"])
			}
			w.Flush()
			return nil
		},
	}
}

// newAddCmd creates add command
func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [id] [name] [ip]",
		Short: "Add a node to the mesh",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			node := map[string]interface{}{
				"id":     args[0],
				"name":   args[1],
				"ip":     args[2],
				"role":   "agent",
				"status": "offline",
			}

			data, _ := json.Marshal(node)
			resp, err := http.Post(
				coordinatorURL+"/api/nodes",
				"application/json",
				bytes.NewReader(data),
			)
			if err != nil {
				return fmt.Errorf("failed to add node: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed to add node: %s", string(body))
			}

			fmt.Printf("Node %s added successfully\n", args[0])
			return nil
		},
	}
}

// newRemoveCmd creates remove command
func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [id]",
		Short: "Remove a node from the mesh",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req, _ := http.NewRequest(
				http.MethodDelete,
				coordinatorURL+"/api/nodes?id="+args[0],
				nil,
			)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to remove node: %w", err)
			}
			defer resp.Body.Close()

			fmt.Printf("Node %s removed successfully\n", args[0])
			return nil
		},
	}
}

// newConfigCmd creates config command
func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show AWG configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get(coordinatorURL + "/api/config")
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}
			defer resp.Body.Close()

			var config map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
				return fmt.Errorf("failed to decode config: %w", err)
			}

			pretty, _ := json.MarshalIndent(config, "", "  ")
			fmt.Println(string(pretty))
			return nil
		},
	}
}

// newStartCmd creates start command
func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [interface]",
		Short: "Start WireGuard interface",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Starting interface %s...\n", args[0])
			// TODO: Implement interface start
			return nil
		},
	}
}

// newStopCmd creates stop command
func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [interface]",
		Short: "Stop WireGuard interface",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Stopping interface %s...\n", args[0])
			// TODO: Implement interface stop
			return nil
		},
	}
}
