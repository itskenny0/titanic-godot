package personalbuild

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/hdpack"
)

func validateHDPlayer(base *zip.ReadCloser, target, root string) error {
	path, err := resolve(root, "manifest.json")
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 16<<20 {
		return fmt.Errorf("HD manifest too large")
	}
	m, err := hdpack.Parse(data)
	if err != nil || !m.Characters {
		return err
	}
	marker := "titanic/hdpack-support.json"
	if target == "android" {
		marker = "assets/hdpack-support.json"
	}
	for _, entry := range base.File {
		if entry.Name != marker {
			continue
		}
		r, err := entry.Open()
		if err != nil {
			return err
		}
		var support struct {
			Characters bool `json:"characters"`
		}
		err = json.NewDecoder(io.LimitReader(r, 4096)).Decode(&support)
		r.Close()
		if err == nil && support.Characters {
			return nil
		}
		break
	}
	return fmt.Errorf("base player does not support HD characters; build a newer base player")
}

func hdInventory(root string) ([]asset, error) {
	path, err := resolve(root, "manifest.json")
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 16<<20 {
		return nil, fmt.Errorf("HD manifest too large")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := hdpack.Parse(b)
	if err != nil {
		return nil, err
	}
	items := []asset{{name: "manifest.json", source: path, size: info.Size(), info: info}}
	keys := make([]string, 0, len(m.Images))
	for k := range m.Images {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Thousands of content-addressed images share one directory. Index it once
	// rather than rescanning it for every case-insensitive lookup.
	imageRoot, err := resolve(root, "images")
	if err != nil {
		return nil, err
	}
	directory, err := os.ReadDir(imageRoot)
	if err != nil {
		return nil, err
	}
	entries := map[string]os.DirEntry{}
	ambiguous := map[string]bool{}
	for _, item := range directory {
		name := strings.ToLower(item.Name())
		if entries[name] != nil {
			ambiguous[name] = true
		}
		entries[name] = item
	}
	for _, key := range keys {
		entry := m.Images[key]
		name := entry.Filename(key)
		base := filepath.Base(name)
		item := entries[base]
		if item == nil || ambiguous[base] {
			return nil, fmt.Errorf("missing or ambiguous HD image: %s", name)
		}
		if !item.Type().IsRegular() {
			return nil, fmt.Errorf("HD image must be a regular file: %s", name)
		}
		p := filepath.Join(imageRoot, item.Name())
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.Size() > hdpack.MaxImageBytes {
			return nil, fmt.Errorf("HD image too large: %s", name)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != entry.SHA256 {
			return nil, fmt.Errorf("HD checksum mismatch: %s", name)
		}
		cfg, err := entry.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if cfg.Width != entry.Width || cfg.Height != entry.Height {
			return nil, fmt.Errorf("HD size mismatch: %s", name)
		}
		items = append(items, asset{name: name, source: p, size: info.Size(), info: info})
	}
	return items, nil
}
func bundleHD(archive, target, root string) error {
	items, err := hdInventory(root)
	if err != nil {
		return err
	}
	src, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	temp := filepath.Join(filepath.Dir(archive), "with-hd.zip")
	out, err := os.Create(temp)
	if err != nil {
		src.Close()
		return err
	}
	z := zip.NewWriter(out)
	prefix := "titanic/hdpack/"
	if target == "android" {
		prefix = "assets/hdpack/"
	}
	for _, entry := range src.File {
		if err = z.Copy(entry); err != nil {
			break
		}
	}
	if err == nil {
		for _, item := range items {
			if err = addAsset(z, prefix, target, item); err != nil {
				break
			}
		}
	}
	zerr := z.Close()
	cerr := out.Close()
	src.Close()
	if err != nil {
		return err
	}
	if zerr != nil {
		return zerr
	}
	if cerr != nil {
		return cerr
	}
	return os.Rename(temp, archive)
}
