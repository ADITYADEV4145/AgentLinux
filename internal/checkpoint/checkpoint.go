// SPDX-License-Identifier: Apache-2.0
// Package checkpoint implements stopped-machine disk checkpoints, never memory snapshots.
// Its caller must hold the lifecycle lock and establish runtime Stopped first.
package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ADITYADEV4145/AgentLinux/internal/protocol"
	"github.com/ADITYADEV4145/AgentLinux/internal/storage"
)

const Recovery = "recovery"

var validName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
var validHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Copy   string `json:"copyMethod"`
}
type Manifest struct {
	Version       int       `json:"version"`
	Name          string    `json:"name"`
	MachineID     string    `json:"machineId"`
	Compatibility string    `json:"compatibility"`
	Created       time.Time `json:"createdAt"`
	Complete      bool      `json:"complete"`
	Files         []File    `json:"files"`
	Status        string    `json:"status,omitempty"`
	directory     string
}
type Journal struct {
	Version       int    `json:"version"`
	MachineID     string `json:"machineId"`
	Compatibility string `json:"compatibility"`
	Kind          string `json:"kind"`
	Phase         string `json:"phase"`
	Name          string `json:"checkpoint"`
	Before        []File `json:"before"`
	Target        []File `json:"target"`
	HelperSHA256  string `json:"helperSha256,omitempty"`
}
type Manager struct {
	Root, MachineID, Compatibility string
	Progress                       func(string)
	Fault                          func(string) error
}

func (m Manager) open() (*os.Root, error) {
	if !protocol.ValidID(m.MachineID) || !validHash.MatchString(m.Compatibility) {
		return nil, errors.New("invalid checkpoint machine/compatibility identity")
	}
	if _, e := storage.ExistingRoot(m.Root); e != nil {
		return nil, e
	}
	return os.OpenRoot(m.Root)
}
func readJSON(root *os.Root, path string, value any) error {
	f, e := safeFile(root, path)
	if e != nil {
		return e
	}
	defer f.Close()
	i, e := f.Stat()
	if e != nil {
		return e
	}
	if i.Mode().Perm()&0077 != 0 {
		return errors.New("checkpoint metadata must be private (0600)")
	}
	if i.Size() > 1<<20 {
		return errors.New("oversized checkpoint metadata")
	}
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if e = d.Decode(value); e != nil {
		return e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("trailing checkpoint metadata")
	}
	return nil
}
func writeJSON(root *os.Root, path string, value any) error {
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	dir := filepath.Dir(path)
	if e = root.MkdirAll(dir, 0700); e != nil {
		return e
	}
	if e = safeParents(root, path); e != nil {
		return e
	}
	if _, e = root.Lstat(path); e == nil {
		f, e := safeFile(root, path)
		if e != nil {
			return e
		}
		f.Close()
	} else if !os.IsNotExist(e) {
		return e
	}
	temp := filepath.Join(dir, ".write-"+protocol.ID())
	f, e := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer root.Remove(temp)
	if _, e = f.Write(append(b, '\n')); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	if e = root.Rename(temp, path); e != nil {
		return e
	}
	return syncAncestors(root, dir)
}
func validateFiles(files []File) error {
	if len(files) != len(paths) {
		return errors.New("incomplete checkpoint file set")
	}
	seen := map[string]bool{}
	for _, f := range files {
		if !allowed(f.Path) || seen[f.Path] || f.Size < 1 || f.Size > maxSize(f.Path) || !validHash.MatchString(f.SHA256) {
			return errors.New("invalid checkpoint file metadata")
		}
		seen[f.Path] = true
	}
	return nil
}
func (m Manager) list(root *os.Root) ([]Manifest, error) {
	dir, e := root.Open("checkpoints")
	if os.IsNotExist(e) {
		return []Manifest{}, nil
	}
	if e != nil {
		return nil, e
	}
	defer dir.Close()
	entries, e := dir.ReadDir(65)
	if e != nil && e != io.EOF {
		return nil, e
	}
	if len(entries) > 16 {
		return nil, errors.New("unexpected checkpoint directory count")
	}
	result := []Manifest{}
	for _, entry := range entries {
		n := entry.Name()
		staged := strings.HasPrefix(n, ".creating-") && protocol.ValidID(strings.TrimPrefix(n, ".creating-"))
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || (!validName.MatchString(n) && !staged) {
			return nil, errors.New("unexpected checkpoint entry; inspect private state")
		}
		var c Manifest
		if e = readJSON(root, filepath.Join("checkpoints", n, "manifest.json"), &c); e != nil {
			if os.IsNotExist(e) && staged {
				result = append(result, Manifest{Version: 1, Name: "interrupted-" + strings.TrimPrefix(n, ".creating-")[:8], MachineID: m.MachineID, Compatibility: m.Compatibility, Status: "incomplete", directory: filepath.Join("checkpoints", n)})
				continue
			}
			return nil, e
		}
		if c.Version != 1 || !validName.MatchString(c.Name) || c.MachineID != m.MachineID || !validHash.MatchString(c.Compatibility) {
			return nil, errors.New("checkpoint identity/schema mismatch")
		}
		if !staged && (!c.Complete || c.Name != n) {
			return nil, errors.New("unfinished checkpoint metadata")
		}
		if c.Complete {
			if e = validateFiles(c.Files); e != nil {
				return nil, e
			}
		}
		c.directory = filepath.Join("checkpoints", n)
		c.Status = "ready"
		if staged {
			c.Status = "incomplete"
		}
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Created.Before(result[j].Created) })
	return result, nil
}
func (m Manager) List() ([]Manifest, error) {
	root, e := m.open()
	if e != nil {
		return nil, e
	}
	defer root.Close()
	return m.list(root)
}
func (m Manager) pending(root *os.Root) (*Journal, error) {
	var j Journal
	receiptError := readJSON(root, "recovery-confirmed.json", &j)
	if receiptError == nil {
		if j.Version != 1 || j.MachineID != m.MachineID || j.Compatibility != m.Compatibility || j.Phase != "confirmed_cleanup" || !validName.MatchString(j.Name) {
			return nil, errors.New("invalid recovery confirmation receipt")
		}
		if j.Kind!="restore"&&j.Kind!="helper-upgrade"&&j.Kind!="rollback"{return nil,errors.New("unknown confirmed recovery kind")}
  if e:=validateFiles(j.Before);e!=nil{return nil,e};if e:=validateFiles(j.Target);e!=nil{return nil,e}
  return &j, nil
 }
 if !os.IsNotExist(receiptError) {
		return nil, receiptError
	}
	e := readJSON(root, "recovery/journal.json", &j)
	if os.IsNotExist(e) {
		if _, e = root.Lstat(Recovery); e == nil {
			return nil, errors.New("recovery directory without journal; inspect before startup")
		}
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if j.Version != 1 || j.MachineID != m.MachineID || j.Compatibility != m.Compatibility || !validName.MatchString(j.Name) {
		return nil, errors.New("recovery identity/compatibility mismatch")
	}
	switch j.Phase {
	case "staging", "installing", "awaiting_ready", "rolling_back", "upgrading_helper":
	default:
		return nil, errors.New("unknown recovery phase")
	}
	if j.Kind != "restore" && j.Kind != "helper-upgrade" && j.Kind != "rollback" {
		return nil, errors.New("unknown recovery kind")
	}
	if j.Phase != "staging" {
		if e = validateFiles(j.Before); e != nil {
			return nil, e
		}
		if e = validateFiles(j.Target); e != nil {
			return nil, e
		}
	}
	return &j, nil
}
func (m Manager) Pending() (*Journal, error) {
	root, e := m.open()
	if e != nil {
		return nil, e
	}
	defer root.Close()
	return m.pending(root)
}
func (m Manager) Create(ctx context.Context, name string) (Manifest, error) {
	var result Manifest
	if !validName.MatchString(name) {
		return result, errors.New("invalid checkpoint name")
	}
	root, e := m.open()
	if e != nil {
		return result, e
	}
	defer root.Close()
	if p, e := m.pending(root); e != nil || p != nil {
		return result, errors.New("resolve pending recovery before creating checkpoints")
	}
	list, e := m.list(root)
	if e != nil {
		return result, e
	}
	if len(list) >= 3 {
		return result, errors.New("three-checkpoint limit reached; explicitly delete one first")
	}
	for _, v := range list {
		if v.Name == name {
			return result, errors.New("checkpoint name already exists")
		}
	}
	result = Manifest{Version: 1, Name: name, MachineID: m.MachineID, Compatibility: m.Compatibility, Created: time.Now().UTC()}
	staging := filepath.Join("checkpoints", ".creating-"+protocol.ID())
	if e = root.MkdirAll(staging, 0700); e != nil {
		return result, e
	}
	if e = writeJSON(root, filepath.Join(staging, "manifest.json"), result); e != nil {
		return result, e
	}
	for _, path := range paths {
		f, e := copyFile(ctx, root, path, filepath.Join(staging, "files", path), path, m.Progress)
		if e != nil {
			return result, fmt.Errorf("checkpoint copy interrupted; delete incomplete checkpoint %s explicitly: %w", name, e)
		}
		result.Files = append(result.Files, f)
	}
	result.Complete = true
	if e = writeJSON(root, filepath.Join(staging, "manifest.json"), result); e != nil {
		return result, e
	}
	if e = root.Rename(staging, filepath.Join("checkpoints", name)); e != nil {
		return result, e
	}
	if e = syncDir(root, "checkpoints"); e != nil {
		return result, e
	}
	result.Status = "ready"
	return result, nil
}
func (m Manager) Delete(name string) error {
	root, e := m.open()
	if e != nil {
		return e
	}
	defer root.Close()
	if p, e := m.pending(root); e != nil || p != nil {
		return errors.New("resolve pending recovery before deleting checkpoints")
	}
	list, e := m.list(root)
	if e != nil {
		return e
	}
	for _, v := range list {
		if v.Name == name {
			if e = validateTree(root, v.directory); e != nil {
				return e
			}
			if e = root.RemoveAll(v.directory); e != nil {
				return e
			}
			return syncDir(root, "checkpoints")
		}
	}
	return errors.New("checkpoint not found")
}

// Only generated names and regular files may be removed from checkpoint/recovery
// directories. Refuse unfamiliar entries instead of silently deleting owner data.
func validateTree(root *os.Root, base string) error {
	var walk func(string) error
	walk = func(path string) error {
		d, e := root.Open(path)
		if e != nil {
			return e
		}
		entries, e := d.ReadDir(128)
		d.Close()
		if e != nil && e != io.EOF {
			return e
		}
		if len(entries) >= 128 {
			return errors.New("unexpected recovery directory size")
		}
		for _, v := range entries {
			p := filepath.Join(path, v.Name())
			if v.Type()&os.ModeSymlink != 0 {
				return errors.New("symlink in recovery tree")
			}
			if v.IsDir() {
				if v.Name() != "files" && v.Name() != "lima" && v.Name() != "default" && v.Name() != "before" && v.Name() != "target" && v.Name() != "undo" {
					return errors.New("unexpected recovery directory")
				}
				if e = walk(p); e != nil {
					return e
				}
			} else {
				if v.Name() != "manifest.json" && v.Name() != "journal.json" && !allowedFileName(v.Name()) && !strings.HasPrefix(v.Name(), ".write-") {
					return errors.New("unexpected recovery file")
				}
				f, e := safeFile(root, p)
				if e != nil {
					return e
				}
				f.Close()
			}
		}
		return nil
	}
	return walk(base)
}
func allowedFileName(name string) bool {
	for _, p := range paths {
		if filepath.Base(p) == name {
			return true
		}
	}
	return false
}
func (m Manager) fault(phase string) error {
	if m.Fault != nil {
		return m.Fault(phase)
	}
	return nil
}
func (m Manager) Restore(ctx context.Context, name string) (*Journal, error) {
	root, e := m.open()
	if e != nil {
		return nil, e
	}
	defer root.Close()
	if p, e := m.pending(root); e != nil || p != nil {
		return nil, errors.New("pending recovery must be explicitly completed or rolled back")
	}
	list, e := m.list(root)
	if e != nil {
		return nil, e
	}
	var chosen *Manifest
	for i := range list {
		if list[i].Name == name && list[i].Status == "ready" {
			chosen = &list[i]
		}
	}
	if chosen == nil {
		return nil, errors.New("complete checkpoint not found")
	}
	if chosen.Compatibility != m.Compatibility {
		return nil, errors.New("checkpoint runtime/VM identity incompatible")
	}
	for _, f := range chosen.Files {
		if e = verify(ctx, root, filepath.Join(chosen.directory, "files"), f); e != nil {
			return nil, e
		}
	}
	j := &Journal{Version: 1, MachineID: m.MachineID, Compatibility: m.Compatibility, Kind: "restore", Phase: "staging", Name: name}
	if e = writeJSON(root, "recovery/journal.json", j); e != nil {
		return nil, e
	}
	if e = m.fault("staging"); e != nil {
		return nil, e
	}
	for _, p := range paths {
		f, e := copyFile(ctx, root, p, filepath.Join(Recovery, "before", p), p, m.Progress)
		if e != nil {
			return nil, e
		}
		j.Before = append(j.Before, f)
	}
	for _, f := range chosen.Files {
		copy, e := copyFile(ctx, root, filepath.Join(chosen.directory, "files", f.Path), filepath.Join(Recovery, "target", f.Path), f.Path, m.Progress)
		if e != nil {
			return nil, e
		}
		if copy.SHA256 != f.SHA256 {
			return nil, errors.New("staged checkpoint integrity mismatch")
		}
		j.Target = append(j.Target, copy)
	}
	if e = syncDir(root, Recovery); e != nil {
		return nil, e
	}
	j.Phase = "installing"
	if e = writeJSON(root, "recovery/journal.json", j); e != nil {
		return nil, e
	}
	if e = m.install(ctx, root, j, "target", j.Target); e != nil {
		return nil, e
	}
	return j, nil
}
func (m Manager) install(ctx context.Context, root *os.Root, j *Journal, stage string, files []File) error {
	for _, f := range files {
		if e := ctx.Err(); e != nil {
			return e
		}
		source := filepath.Join(Recovery, stage, f.Path)
		if _, e := root.Lstat(source); os.IsNotExist(e) { // Already swapped before interruption.
			if e = verify(ctx, root, ".", f); e != nil {
				return e
			}
			continue
		}
		if e := verify(ctx, root, filepath.Join(Recovery, stage), f); e != nil {
			return e
		}
		live, e := safeFile(root, f.Path)
		if e != nil {
			return e
		}
		live.Close()
		if e = root.Rename(source, f.Path); e != nil {
			return e
		}
		if e = syncDir(root, filepath.Dir(f.Path)); e != nil {
			return e
		}
		if e = m.fault("installed:" + f.Path); e != nil {
			return e
		}
	}
	j.Phase = "awaiting_ready"
	return writeJSON(root, "recovery/journal.json", j)
}
func (m Manager) Recover(ctx context.Context, rollback bool) error {
	root, e := m.open()
	if e != nil {
		return e
	}
	defer root.Close()
	j, e := m.pending(root)
	if e != nil {
		return e
	}
	if j == nil {
		return errors.New("no pending recovery")
	}
	if j.Phase == "confirmed_cleanup" {
		if rollback {
			return errors.New("rollback already released after verified readiness; choose resume to finish cleanup")
		}
		return m.cleanupConfirmed(root)
	}
	if j.Phase == "staging" { // Originals cannot have been changed before installing was synced.
		if !rollback {
			return errors.New("staging was interrupted; explicitly choose rollback to discard incomplete staging")
		}
		if e = validateTree(root, Recovery); e != nil {
			return e
		}
		return root.RemoveAll(Recovery)
	}
	if !rollback {
		if j.Phase != "installing" {
			return errors.New("choose rollback or boot explicitly; automatic restart is disabled")
		}
		return m.install(ctx, root, j, "target", j.Target)
	}
	for _, f := range j.Before {
		if e = verify(ctx, root, "recovery/before", f); e != nil {
			return e
		}
	}
	if _, e = root.Lstat("recovery/undo"); e == nil {
		if e = validateTree(root, "recovery/undo"); e != nil {
			return e
		}
		if e = root.RemoveAll("recovery/undo"); e != nil {
			return e
		}
	}
	for _, f := range j.Before {
		copy, e := copyFile(ctx, root, filepath.Join("recovery/before", f.Path), filepath.Join("recovery/undo", f.Path), f.Path, m.Progress)
		if e != nil {
			return e
		}
		if copy.SHA256 != f.SHA256 {
			return errors.New("rollback integrity mismatch")
		}
	}
	j.Phase = "rolling_back"
	j.Kind = "rollback"
	j.HelperSHA256 = ""
	j.Target = j.Before
	if e = writeJSON(root, "recovery/journal.json", j); e != nil {
		return e
	}
	return m.install(ctx, root, j, "undo", j.Before)
}
func (m Manager) Confirm() error {
	root, e := m.open()
	if e != nil {
		return e
	}
	defer root.Close()
	j, e := m.pending(root)
	if e != nil {
		return e
	}
	if j == nil {
		return nil
	}
	if j.Phase != "awaiting_ready" && j.Phase != "confirmed_cleanup" {
		return errors.New("recovery not ready to confirm")
	}
	j.Phase = "confirmed_cleanup"
	if e = writeJSON(root, "recovery-confirmed.json", j); e != nil {
		return e
	}
	if e = m.fault("confirmed"); e != nil {
		return e
	}
	return m.cleanupConfirmed(root)
}
func (m Manager) cleanupConfirmed(root *os.Root) error {
	if _, e := root.Lstat(Recovery); e == nil {
		if e = validateTree(root, Recovery); e != nil {
			return e
		}
		if e = root.RemoveAll(Recovery); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if e := syncDir(root, "."); e != nil {
		return e
	}
	if e := m.fault("cleanup:recovery"); e != nil {
		return e
	}
	if e := root.Remove("recovery-confirmed.json"); e != nil && !os.IsNotExist(e) {
		return e
	}
	return syncDir(root, ".")
}

func (m Manager) RequireHelper(digest string) error {
	if !validHash.MatchString(digest) {
		return errors.New("invalid helper digest")
	}
	root, e := m.open()
	if e != nil {
		return e
	}
	defer root.Close()
	j, e := m.pending(root)
	if e != nil {
		return e
	}
	if j == nil || j.Phase != "awaiting_ready" {
		return errors.New("upgrade requires staged checkpoint-backed recovery")
	}
	j.Kind = "helper-upgrade"
	j.Phase = "upgrading_helper"
	j.HelperSHA256 = digest
	return writeJSON(root, "recovery/journal.json", j)
}

func (m Manager) UpgradeInstalled() error {
	root, e := m.open()
	if e != nil {
		return e
	}
	defer root.Close()
	j, e := m.pending(root)
	if e != nil {
		return e
	}
	if j == nil || j.Phase != "upgrading_helper" {
		return errors.New("no helper metadata upgrade in progress")
	}
	j.Phase = "awaiting_ready"
	return writeJSON(root, "recovery/journal.json", j)
}
