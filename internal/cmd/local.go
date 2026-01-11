package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type localAPI interface {
	Undo() error
	Redo() error
	ReorderBlocks(parentUID string, blockUIDs []string) error
	UploadFile(filename string, data []byte) (string, error)
	DownloadFile(url string) ([]byte, error)
	DeleteFile(url string) error
	AddPageShortcut(uid string, index *int) error
	RemovePageShortcut(uid string) error
	UpsertUser(userUID, displayName string) error
	Call(action string, args ...interface{}) (json.RawMessage, error)
}

var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Local API operations (requires Roam desktop app)",
	Long: `Commands that use the Local HTTP API.

These commands require the Roam desktop app to be running with
'Encrypted local API' enabled in settings.`,
}

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo the last action",
	Long:  `Undo the last action in the graph using the Local API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		if err := client.Undo(); err != nil {
			return fmt.Errorf("undo failed: %w", err)
		}

		if structuredOutputRequested() {
			return printStructured(map[string]interface{}{
				"status": "success",
				"action": "undo",
			})
		}
		fmt.Println("Undo successful")
		return nil
	},
}

var redoCmd = &cobra.Command{
	Use:   "redo",
	Short: "Redo the last undone action",
	Long:  `Redo the last undone action in the graph using the Local API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		if err := client.Redo(); err != nil {
			return fmt.Errorf("redo failed: %w", err)
		}

		if structuredOutputRequested() {
			return printStructured(map[string]interface{}{
				"status": "success",
				"action": "redo",
			})
		}
		fmt.Println("Redo successful")
		return nil
	},
}

var reorderCmd = &cobra.Command{
	Use:   "reorder <parent-uid> <block-uid>...",
	Short: "Reorder child blocks under a parent",
	Long: `Reorder child blocks under a parent block in the specified order.

The block UIDs should be listed in the desired order.

Example:
  roam local reorder parent123 child1 child2 child3`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		parentUID := args[0]
		blockUIDs := args[1:]

		if err := client.ReorderBlocks(parentUID, blockUIDs); err != nil {
			return fmt.Errorf("reorder failed: %w", err)
		}

		if structuredOutputRequested() {
			return printStructured(map[string]interface{}{
				"status":     "success",
				"parent_uid": parentUID,
				"count":      len(blockUIDs),
			})
		}
		fmt.Printf("Reordered %d blocks under %s\n", len(blockUIDs), parentUID)
		return nil
	},
}

var uploadFileCmd = &cobra.Command{
	Use:   "upload <file>",
	Short: "Upload a file to the graph",
	Long: `Upload a file to the graph and get a URL that can be embedded.

Example:
  roam local upload image.png`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		filename := args[0]

		// Read file
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Extract just the filename without path
		parts := strings.Split(filename, "/")
		baseName := parts[len(parts)-1]

		url, err := client.UploadFile(baseName, data)
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		if structuredOutputRequested() {
			return printStructured(map[string]interface{}{
				"status": "success",
				"url":    url,
				"file":   baseName,
			})
		}
		fmt.Printf("Uploaded: %s\n", url)
		return nil
	},
}

var downloadFileCmd = &cobra.Command{
	Use:   "download <url> [output-file]",
	Short: "Download a file from the graph",
	Long: `Download a file from a Roam storage URL.

If output-file is not specified, writes to stdout.

Example:
  roam local download "https://firebasestorage.googleapis.com/..." image.png`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		url := args[0]

		data, err := client.DownloadFile(url)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		if len(args) > 1 {
			outputFile := args[1]
			if err := os.WriteFile(outputFile, data, 0o644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			if structuredOutputRequested() {
				return printStructured(map[string]interface{}{
					"status": "success",
					"output": outputFile,
				})
			}
			fmt.Printf("Downloaded to: %s\n", outputFile)
		} else {
			// Write to stdout
			_, err := io.Copy(os.Stdout, bytes.NewReader(data))
			return err
		}

		return nil
	},
}

var deleteFileCmd = &cobra.Command{
	Use:   "delete <url>",
	Short: "Delete a file from the graph",
	Long: `Delete a file from a Roam storage URL.

Example:
  roam local delete "https://firebasestorage.googleapis.com/..."`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		url := args[0]

		if err := client.DeleteFile(url); err != nil {
			return fmt.Errorf("delete failed: %w", err)
		}

		if structuredOutputRequested() {
			return printStructured(map[string]interface{}{
				"status": "success",
				"url":    url,
			})
		}
		fmt.Printf("Deleted: %s\n", url)
		return nil
	},
}

var shortcutCmd = &cobra.Command{
	Use:   "shortcut",
	Short: "Manage page shortcuts",
	Long:  `Add or remove pages from the left sidebar shortcuts using the Local API.`,
}

var shortcutAddCmd = &cobra.Command{
	Use:   "add <page-uid> [index]",
	Short: "Add a page shortcut",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		uid := args[0]
		var idx *int
		if len(args) == 2 {
			parsed, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid index: %w", err)
			}
			idx = &parsed
		}

		if err := client.AddPageShortcut(uid, idx); err != nil {
			return fmt.Errorf("add shortcut failed: %w", err)
		}

		if structuredOutputRequested() {
			result := map[string]interface{}{
				"status":   "success",
				"page_uid": uid,
			}
			if idx != nil {
				result["index"] = *idx
			}
			return printStructured(result)
		}
		fmt.Printf("Shortcut added: %s\n", uid)
		return nil
	},
}

var shortcutRemoveCmd = &cobra.Command{
	Use:   "remove <page-uid>",
	Short: "Remove a page shortcut",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		uid := args[0]
		if err := client.RemovePageShortcut(uid); err != nil {
			return fmt.Errorf("remove shortcut failed: %w", err)
		}

		if structuredOutputRequested() {
			return printStructured(map[string]interface{}{
				"status":   "success",
				"page_uid": uid,
			})
		}
		fmt.Printf("Shortcut removed: %s\n", uid)
		return nil
	},
}

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User operations",
	Long:  `Manage user entities using the Local API.`,
}

var userUpsertCmd = &cobra.Command{
	Use:   "upsert <user-uid>",
	Short: "Create or update a user entity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		userUID := args[0]
		displayName, _ := cmd.Flags().GetString("display-name")

		if err := client.UpsertUser(userUID, displayName); err != nil {
			return fmt.Errorf("upsert user failed: %w", err)
		}

		if structuredOutputRequested() {
			result := map[string]interface{}{
				"status":    "success",
				"user_uid":  userUID,
				"display":   displayName,
				"operation": "upsert",
			}
			return printStructured(result)
		}
		fmt.Printf("User upserted: %s\n", userUID)
		return nil
	},
}

var (
	localCallArgs     string
	localCallArgsFile string
)

var localCallCmd = &cobra.Command{
	Use:   "call <action>",
	Short: "Call an arbitrary Local API action",
	Long: `Invoke any Local API action by name with JSON arguments.

Examples:
  roam local call data.q --args '["[:find ?t :where [?e :node/title ?t]]"]'
  roam local call ui.rightSidebar.open --args '[]'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getLocalClient()
		if err != nil {
			return err
		}

		action := args[0]

		var raw []byte
		if localCallArgsFile != "" {
			raw, err = os.ReadFile(localCallArgsFile)
			if err != nil {
				return fmt.Errorf("failed to read args file: %w", err)
			}
		} else if localCallArgs != "" {
			raw = []byte(localCallArgs)
		}

		callArgs := []interface{}{}
		if len(raw) > 0 {
			var parsed interface{}
			if err := json.Unmarshal(raw, &parsed); err != nil {
				return fmt.Errorf("invalid args JSON: %w", err)
			}
			switch v := parsed.(type) {
			case []interface{}:
				callArgs = v
			default:
				callArgs = []interface{}{v}
			}
		}

		result, err := client.Call(action, callArgs...)
		if err != nil {
			return fmt.Errorf("local call failed: %w", err)
		}

		if structuredOutputRequested() {
			var decoded interface{}
			if len(result) > 0 {
				_ = json.Unmarshal(result, &decoded)
			}
			return printStructured(map[string]interface{}{
				"status": "success",
				"action": action,
				"result": decoded,
			})
		}

		if len(result) > 0 {
			fmt.Println(string(result))
		} else {
			fmt.Println("OK")
		}
		return nil
	},
}

// getLocalClient creates a LocalClient for the current graph
func getLocalClient() (localAPI, error) {
	name, err := getGraphNameForLocal()
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("graph name required (set ROAM_GRAPH_NAME)")
	}
	return newLocalClientFunc(name)
}

// getGraphNameForLocal gets the graph name from flags, environment, or keyring
func getGraphNameForLocal() (string, error) {
	if graphName != "" {
		return graphName, nil
	}
	if g := envGet("ROAM_GRAPH_NAME"); g != "" {
		return g, nil
	}
	cfg, err := loadConfigFromFlag()
	if err != nil {
		return "", formatConfigLoadError(err)
	}
	if cfg != nil && strings.TrimSpace(cfg.GraphName) != "" {
		return strings.TrimSpace(cfg.GraphName), nil
	}
	// Try to get from keyring
	store, err := openSecretsStore()
	if err == nil {
		if tok, err := store.GetToken(graphKeyPrefix + defaultProfile); err == nil {
			return tok.RefreshToken, nil
		}
	}
	return "", nil
}

func init() {
	rootCmd.AddCommand(localCmd)

	localCmd.AddCommand(undoCmd)
	localCmd.AddCommand(redoCmd)
	localCmd.AddCommand(reorderCmd)
	localCmd.AddCommand(uploadFileCmd)
	localCmd.AddCommand(downloadFileCmd)
	localCmd.AddCommand(deleteFileCmd)
	localCmd.AddCommand(shortcutCmd)
	shortcutCmd.AddCommand(shortcutAddCmd)
	shortcutCmd.AddCommand(shortcutRemoveCmd)
	localCmd.AddCommand(userCmd)
	userCmd.AddCommand(userUpsertCmd)
	localCmd.AddCommand(localCallCmd)

	userUpsertCmd.Flags().String("display-name", "", "User display name")
	localCallCmd.Flags().StringVar(&localCallArgs, "args", "", "JSON array/object of args")
	localCallCmd.Flags().StringVar(&localCallArgsFile, "args-file", "", "Path to JSON args file")
}
