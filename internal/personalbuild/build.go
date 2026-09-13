// Package personalbuild creates local packages containing the owner's game data.
package personalbuild

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/itskenny0/titanic-godot/internal/patches"
)

type Options struct {
	Target, Base, GameData, Output, Manifest, BuildTools, Keystore, StripTool, PatchArchive string
}

type asset struct {
	name, source string
	size         int64
	info         os.FileInfo
}

// resolve rejects links and ambiguous case variants, including directory components.
func resolve(root, relative string) (string, error) {
	current := root
	for _, part := range strings.Split(relative, "/") {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", err
		}
		found := ""
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), part) {
				if found != "" {
					return "", fmt.Errorf("ambiguous name %s in %s", part, current)
				}
				if entry.Type()&os.ModeSymlink != 0 {
					return "", fmt.Errorf("game data contains a symbolic link: %s", filepath.Join(current, entry.Name()))
				}
				found = entry.Name()
			}
		}
		if found == "" {
			return "", fmt.Errorf("missing required game file: %s", relative)
		}
		current = filepath.Join(current, found)
	}
	return current, nil
}

func inventory(root, manifest string) ([]asset, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("game data must be a prepared folder containing cd1 and cd2")
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		return nil, err
	}
	var required map[string][]string
	if err = json.Unmarshal(data, &required); err != nil {
		return nil, err
	}
	if len(required) != 2 || len(required["1"]) == 0 || len(required["2"]) == 0 {
		return nil, fmt.Errorf("invalid required-file manifest")
	}
	var result []asset
	seen := map[string]bool{}
	for _, disc := range []string{"1", "2"} {
		for _, relative := range required[disc] {
			if !safePath(relative) || relative != strings.ToLower(relative) || strings.HasSuffix(relative, ".ti") {
				return nil, fmt.Errorf("unsafe game asset in manifest: %q", relative)
			}
			name := "cd" + disc + "/" + relative
			if seen[name] {
				return nil, fmt.Errorf("duplicate asset: %s", name)
			}
			seen[name] = true
			source, err := resolve(root, name)
			if err != nil {
				return nil, err
			}
			info, err := os.Stat(source)
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() || info.Size() == 0 {
				return nil, fmt.Errorf("empty or nonregular game asset: %s", source)
			}
			result = append(result, asset{name, source, info.Size(), info})
		}
	}
	// Check the same identifying DreamFactory containers as prepare-game-data.py.
	for _, name := range []string{"cd1/data/bootfile", "cd1/data/bedsit1.set", "cd1/data/main.stg", "cd1/data/ctl.stg", "cd2/data/a14.set", "cd2/data/deckbd.set", "cd2/data/cargo.set"} {
		source, err := resolve(root, name)
		if err != nil {
			return nil, err
		}
		f, err := os.Open(source)
		if err != nil {
			return nil, err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, err
		}
		var header [32]byte
		_, err = io.ReadFull(f, header[:])
		f.Close()
		valid := false
		if err == nil && info.Size() >= 1024 {
			for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
				count := int64(order.Uint32(header[20:]))
				valid = valid || int64(order.Uint32(header[4:])) == info.Size() && count > 0 && count <= (info.Size()-1024)/4
			}
		}
		if !valid {
			return nil, fmt.Errorf("damaged DreamFactory container: %s", source)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result, nil
}

func safePath(name string) bool {
	return name != "" && !strings.ContainsAny(name, "\\:\x00") && !strings.HasPrefix(name, "/") && path.Clean(name) == strings.TrimSuffix(name, "/") && name != "." && name != ".." && !strings.HasPrefix(name, "../")
}

// Android's _cl_ is a little-endian count followed by length-prefixed UTF-8 args.
func bundledArgs(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	var count uint32
	if binary.Read(reader, binary.LittleEndian, &count) != nil || count > 4096 {
		return nil, fmt.Errorf("invalid Android command line")
	}
	args := []string{}
	hasSeparator := false
	for range count {
		var size uint32
		if binary.Read(reader, binary.LittleEndian, &size) != nil || uint64(size) > uint64(reader.Len()) {
			return nil, fmt.Errorf("truncated Android command line")
		}
		value := make([]byte, size)
		io.ReadFull(reader, value)
		arg := string(value)
		if strings.HasPrefix(arg, "--game-data=") {
			continue
		}
		hasSeparator = hasSeparator || arg == "--"
		args = append(args, arg)
	}
	if reader.Len() != 0 {
		return nil, fmt.Errorf("trailing Android command line data")
	}
	if !hasSeparator {
		args = append(args, "--")
	}
	args = append(args, "--game-data=res://gamedata")
	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint32(len(args)))
	for _, arg := range args {
		binary.Write(&out, binary.LittleEndian, uint32(len(arg)))
		out.WriteString(arg)
	}
	return out.Bytes(), nil
}

func signature(name string) bool {
	name = strings.ToUpper(name)
	if !strings.HasPrefix(name, "META-INF/") {
		return false
	}
	ext := path.Ext(name)
	return name == "META-INF/MANIFEST.MF" || ext == ".SF" || ext == ".RSA" || ext == ".DSA" || ext == ".EC"
}

func rewrite(base *zip.ReadCloser, out io.Writer, target string, assets []asset, stripTool, stage string, extraPatches ...[]asset) error {
	writer := zip.NewWriter(out)
	prefix := "titanic/gamedata/"
	if target == "android" {
		prefix = "assets/gamedata/"
	}
	patchPrefix := "titanic/patches/files/"
	if target == "android" {
		patchPrefix = "assets/patches/files/"
	}
	seen := map[string]bool{}
	for _, entry := range base.File {
		if !safePath(entry.Name) || seen[entry.Name] || entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe or duplicate package entry: %s", entry.Name)
		}
		seen[entry.Name] = true
		if len(extraPatches) > 0 && strings.HasPrefix(entry.Name, patchPrefix) {
			continue
		}
		if strings.HasPrefix(entry.Name, prefix) {
			if entry.Name == prefix || entry.Name == prefix+"PUT_GAME_FILES_HERE.txt" {
				continue
			}
			return fmt.Errorf("base already contains game data; use an unbundled package")
		}
		if target == "android" && signature(entry.Name) {
			continue
		}
		if target == "android" && entry.Name == "assets/_cl_" {
			if entry.UncompressedSize64 > 65536 {
				return fmt.Errorf("Android command line is too large")
			}
			stream, err := entry.Open()
			if err != nil {
				return err
			}
			data, err := io.ReadAll(io.LimitReader(stream, 65537))
			stream.Close()
			if err != nil {
				return err
			}
			data, err = bundledArgs(data)
			if err != nil {
				return err
			}
			dst, err := writer.CreateHeader(&zip.FileHeader{Name: entry.Name, Method: zip.Store})
			if err != nil {
				return err
			}
			if _, err = dst.Write(data); err != nil {
				return err
			}
		} else if stripTool != "" && strings.HasSuffix(entry.Name, ".so") && (strings.HasPrefix(entry.Name, "lib/") || strings.HasPrefix(entry.Name, "titanic/native/")) {
			if err := strippedLibrary(writer, entry, stripTool, stage); err != nil {
				return err
			}
		} else if err := writer.Copy(entry); err != nil {
			return err
		}
	}
	for _, item := range assets {
		if err := addAsset(writer, prefix, target, item); err != nil {
			return err
		}
	}
	for _, group := range extraPatches {
		for _, item := range group {
			if err := addAsset(writer, patchPrefix, target, item); err != nil {
				return err
			}
		}
	}
	return writer.Close()
}

func addAsset(writer *zip.Writer, prefix, target string, item asset) error {
	file, err := os.Open(item.source)
	if err != nil {
		return err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(item.info, before) || before.Size() != item.size || !before.ModTime().Equal(item.info.ModTime()) {
		return fmt.Errorf("source changed: %s", item.source)
	}
	header := &zip.FileHeader{Name: prefix + item.name, Method: zip.Deflate}
	header.SetMode(0644)
	// Android must seek directly in large movie and sound assets without inflating them.
	if target == "android" {
		header.Method = zip.Store
	}
	dst, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	count, err := io.Copy(dst, file)
	if err != nil {
		return err
	}
	after, err := file.Stat()
	if err != nil {
		return err
	}
	if count != item.size || !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		return fmt.Errorf("source changed while copying: %s", item.source)
	}
	return nil
}

func runTool(tool string, args ...string) error {
	// Invoke the SDK's jar directly on Windows instead of passing user paths
	// through cmd.exe to run apksigner.bat.
	if strings.EqualFold(filepath.Base(tool), "apksigner.bat") {
		args = append([]string{"-jar", filepath.Join(filepath.Dir(tool), "lib", "apksigner.jar")}, args...)
		tool = "java"
		if home := os.Getenv("JAVA_HOME"); home != "" {
			tool = filepath.Join(home, "bin", "java")
		}
	}
	cmd := exec.Command(tool, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(tool), err)
	}
	return nil
}

func Build(options Options) error {
	return build(options, patches.Pinned())
}

func build(options Options, patchManifest patches.Manifest) error {
	if options.Target != "android" && options.Target != "portmaster" {
		return fmt.Errorf("target must be android or portmaster")
	}
	if options.Base == "" || options.GameData == "" || options.Output == "" {
		return fmt.Errorf("base, game-data and output are required")
	}
	root, err := filepath.Abs(options.GameData)
	if err != nil {
		return err
	}
	output, err := filepath.Abs(options.Output)
	if err != nil {
		return err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(realRoot, parent)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("output must be outside the game-data folder")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return fmt.Errorf("output already exists or cannot be checked: %s", output)
	}
	assets, err := inventory(root, options.Manifest)
	if err != nil {
		return err
	}
	base, err := zip.OpenReader(options.Base)
	if err != nil {
		return err
	}
	defer base.Close()
	entries := map[string]bool{}
	for _, file := range base.File {
		entries[file.Name] = true
	}
	required := []string{"Titanic.sh", "titanic/titanic.pck", "titanic/native/libtitanic.aarch64.so"}
	if options.Target == "android" {
		required = []string{"AndroidManifest.xml", "assets/_cl_", "assets/required_files.json", "lib/arm64-v8a/libtitanic_go.so"}
	}
	for _, name := range required {
		if !entries[name] {
			return fmt.Errorf("base is not a native Go %s package: missing %s", options.Target, name)
		}
	}
	if options.StripTool != "" {
		if _, err := exec.LookPath(options.StripTool); err != nil {
			return err
		}
	}
	var zipalign, apksigner string
	if options.Target == "android" {
		zipalign, apksigner = "zipalign", "apksigner"
		if options.BuildTools != "" {
			zipalign = filepath.Join(options.BuildTools, zipalign)
			apksigner = filepath.Join(options.BuildTools, apksigner)
		}
		if _, err := exec.LookPath(zipalign); err != nil {
			return fmt.Errorf("install Android SDK Build Tools and set --android-build-tools: %w", err)
		}
		apksigner, err = exec.LookPath(apksigner)
		if err != nil {
			return err
		}
		if _, err := os.Stat(options.Keystore); err != nil {
			return err
		}
		if err := runTool(apksigner, "verify", options.Base); err != nil {
			return err
		}
	}
	stage, err := os.MkdirTemp(parent, ".titanic-personal-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	patchTarget := filepath.Join(stage, "patches")
	if err := patches.Install(context.Background(), patches.Request{Archive: options.PatchArchive, Target: patchTarget}, patchManifest, nil); err != nil {
		return fmt.Errorf("bundling personal patches: %w", err)
	}
	patchAssets := []asset{}
	for name := range patchManifest.Files {
		source := filepath.Join(patchTarget, name)
		info, err := os.Stat(source)
		if err != nil {
			return err
		}
		patchAssets = append(patchAssets, asset{name, source, info.Size(), info})
	}
	sort.Slice(patchAssets, func(i, j int) bool { return patchAssets[i].name < patchAssets[j].name })
	unsigned := filepath.Join(stage, "unsigned.zip")
	file, err := os.Create(unsigned)
	if err != nil {
		return err
	}
	err = rewrite(base, file, options.Target, assets, options.StripTool, stage, patchAssets)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	result := unsigned
	if options.Target == "android" {
		aligned, signed := filepath.Join(stage, "aligned.apk"), filepath.Join(stage, "signed.apk")
		if err := runTool(zipalign, "-P", "16", "-f", "4", unsigned, aligned); err != nil {
			return err
		}
		if err := runTool(apksigner, "sign", "--ks", options.Keystore, "--ks-key-alias", "androiddebugkey", "--ks-pass", "pass:android", "--key-pass", "pass:android", "--out", signed, aligned); err != nil {
			return err
		}
		if err := runTool(apksigner, "verify", signed); err != nil {
			return err
		}
		if err := runTool(zipalign, "-c", "-P", "16", "4", signed); err != nil {
			return err
		}
		result = signed
	}
	// O_EXCL preserves existing packages, originals and saves even if a path appears mid-build.
	source, err := os.Open(result)
	if err != nil {
		return err
	}
	defer source.Close()
	dest, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dest, source)
	closeErr = dest.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(output)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	fmt.Printf("Built %s with %d game files. Keep this personal package private.\n", output, len(assets))
	return nil
}

// LLVM strip understands both PortMaster ARM architectures and Android ELF.
// Keep dynamic exports, relocations and Go's runtime tables; remove debug symbols.
func strippedLibrary(writer *zip.Writer, entry *zip.File, tool, stage string) error {
	stream, err := entry.Open()
	if err != nil {
		return err
	}
	defer stream.Close()
	file, err := os.CreateTemp(stage, "native-*.so")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	_, copyErr := io.Copy(file, stream)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := runTool(tool, "--strip-unneeded", name); err != nil {
		return err
	}
	stripped, err := os.Open(name)
	if err != nil {
		return err
	}
	defer stripped.Close()
	header := &zip.FileHeader{Name: entry.Name, Method: entry.Method, Modified: entry.Modified}
	header.SetMode(entry.Mode())
	dst, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, stripped)
	return err
}
