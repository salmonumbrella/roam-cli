package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/salmonumbrella/roam-cli/internal/api"
	"github.com/salmonumbrella/roam-cli/internal/output"
	"github.com/salmonumbrella/roam-cli/internal/secrets"
)

type fakeStore struct {
	setTokens    map[string]secrets.Token
	deletedKeys  []string
	defaultAcct  string
	listTokens   []secrets.Token
	getTokenFunc func(string) (secrets.Token, error)
}

func (f *fakeStore) Keys() ([]string, error) { return nil, nil }
func (f *fakeStore) SetToken(profile string, tok secrets.Token) error {
	if f.setTokens == nil {
		f.setTokens = make(map[string]secrets.Token)
	}
	f.setTokens[profile] = tok
	return nil
}

func (f *fakeStore) GetToken(profile string) (secrets.Token, error) {
	if f.getTokenFunc != nil {
		return f.getTokenFunc(profile)
	}
	if tok, ok := f.setTokens[profile]; ok {
		return tok, nil
	}
	return secrets.Token{}, errors.New("not found")
}

func (f *fakeStore) DeleteToken(profile string) error {
	f.deletedKeys = append(f.deletedKeys, profile)
	return nil
}
func (f *fakeStore) ListTokens() ([]secrets.Token, error) { return f.listTokens, nil }
func (f *fakeStore) GetDefaultAccount() (string, error)   { return f.defaultAcct, nil }
func (f *fakeStore) SetDefaultAccount(profile string) error {
	f.defaultAcct = profile
	return nil
}

func TestAuthLoginLogoutStatus(t *testing.T) {
	store := &fakeStore{}
	prevOpen := openSecretsStore
	openSecretsStore = func() (secrets.Store, error) { return store, nil }
	defer func() { openSecretsStore = prevOpen }()

	prevNewClient := newClientFromCredsFunc
	newClientFromCredsFunc = func(graphName, token, mode string, opts ...api.ClientOption) (api.RoamAPI, error) {
		return &fakeClient{
			QueryFunc: func(query string, args ...interface{}) ([][]interface{}, error) {
				return [][]interface{}{{"ok"}}, nil
			},
		}, nil
	}
	defer func() { newClientFromCredsFunc = prevNewClient }()

	loginToken = "tok"
	loginGraph = "graph"
	encryptedGraph = false
	defer func() {
		loginToken = ""
		loginGraph = ""
		encryptedGraph = false
	}()

	_, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()
	setCmdContext(loginCmd)
	if err := runLogin(loginCmd, []string{}); err != nil {
		t.Fatalf("runLogin failed: %v", err)
	}

	if len(store.setTokens) == 0 {
		t.Fatal("expected token stored")
	}

	setCmdContext(logoutCmd)
	if err := runLogout(logoutCmd, []string{}); err != nil {
		t.Fatalf("runLogout failed: %v", err)
	}
	if len(store.deletedKeys) == 0 {
		t.Fatal("expected delete token")
	}

	// Status with structured output
	store.setTokens[defaultProfile] = secrets.Token{Profile: defaultProfile, RefreshToken: "tok", CreatedAt: time.Now()}
	store.setTokens[graphKeyPrefix+defaultProfile] = secrets.Token{Profile: graphKeyPrefix + defaultProfile, RefreshToken: "graph"}
	prevVerify := verifyAuth
	verifyAuth = false
	defer func() { verifyAuth = prevVerify }()
	setCmdContext(statusCmd)
	if err := runStatus(statusCmd, []string{}); err != nil {
		t.Fatalf("runStatus failed: %v", err)
	}
}

type fakeLocal struct {
	undoCalls    int
	redoCalls    int
	reorderCalls int
	uploadURL    string
	downloadData []byte
	deletedURL   string
	shortcuts    []string
	userUID      string
}

func (f *fakeLocal) Undo() error {
	f.undoCalls++
	return nil
}

func (f *fakeLocal) Redo() error {
	f.redoCalls++
	return nil
}

func (f *fakeLocal) ReorderBlocks(parentUID string, blockUIDs []string) error {
	f.reorderCalls++
	return nil
}

func (f *fakeLocal) UploadFile(filename string, data []byte) (string, error) {
	f.uploadURL = "https://file"
	return f.uploadURL, nil
}

func (f *fakeLocal) DownloadFile(url string) ([]byte, error) {
	f.downloadData = []byte("data")
	return f.downloadData, nil
}

func (f *fakeLocal) DeleteFile(url string) error {
	f.deletedURL = url
	return nil
}

func (f *fakeLocal) AddPageShortcut(uid string, index *int) error {
	f.shortcuts = append(f.shortcuts, uid)
	return nil
}

func (f *fakeLocal) RemovePageShortcut(uid string) error {
	f.shortcuts = append(f.shortcuts, uid)
	return nil
}

func (f *fakeLocal) UpsertUser(userUID, displayName string) error {
	f.userUID = userUID
	return nil
}

func (f *fakeLocal) Call(action string, args ...interface{}) (json.RawMessage, error) {
	return json.RawMessage(`{"ok":true}`), nil
}

func TestLocalCommands(t *testing.T) {
	local := &fakeLocal{}
	prevNewLocal := newLocalClientFunc
	newLocalClientFunc = func(graphName string) (localAPI, error) { return local, nil }
	defer func() { newLocalClientFunc = prevNewLocal }()

	graphName = "graph"
	defer func() { graphName = "" }()

	out, _, restoreCtx := withTestContext(t, output.FormatJSON, true)
	defer restoreCtx()

	setCmdContext(undoCmd)
	if err := undoCmd.RunE(undoCmd, []string{}); err != nil {
		t.Fatalf("undo failed: %v", err)
	}

	setCmdContext(redoCmd)
	if err := redoCmd.RunE(redoCmd, []string{}); err != nil {
		t.Fatalf("redo failed: %v", err)
	}

	setCmdContext(reorderCmd)
	if err := reorderCmd.RunE(reorderCmd, []string{"parent", "child1"}); err != nil {
		t.Fatalf("reorder failed: %v", err)
	}

	// upload/download/delete
	fileData := []byte("file")
	filePath := t.TempDir() + "/file.txt"
	if err := os.WriteFile(filePath, fileData, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	setCmdContext(uploadFileCmd)
	if err := uploadFileCmd.RunE(uploadFileCmd, []string{filePath}); err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	out.Reset()
	setCmdContext(downloadFileCmd)
	if err := downloadFileCmd.RunE(downloadFileCmd, []string{"https://file"}); err != nil {
		t.Fatalf("download failed: %v", err)
	}

	setCmdContext(deleteFileCmd)
	if err := deleteFileCmd.RunE(deleteFileCmd, []string{"https://file"}); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	setCmdContext(shortcutAddCmd)
	if err := shortcutAddCmd.RunE(shortcutAddCmd, []string{"page-uid"}); err != nil {
		t.Fatalf("shortcut add failed: %v", err)
	}

	setCmdContext(shortcutRemoveCmd)
	if err := shortcutRemoveCmd.RunE(shortcutRemoveCmd, []string{"page-uid"}); err != nil {
		t.Fatalf("shortcut remove failed: %v", err)
	}

	setCmdContext(userUpsertCmd)
	if err := userUpsertCmd.Flags().Set("display-name", "User"); err != nil {
		t.Fatalf("set display name: %v", err)
	}
	if err := userUpsertCmd.RunE(userUpsertCmd, []string{"user-uid"}); err != nil {
		t.Fatalf("user upsert failed: %v", err)
	}

	setCmdContext(localCallCmd)
	localCallArgs = "[]"
	defer func() { localCallArgs = "" }()
	if err := localCallCmd.RunE(localCallCmd, []string{"action"}); err != nil {
		t.Fatalf("local call failed: %v", err)
	}
}
