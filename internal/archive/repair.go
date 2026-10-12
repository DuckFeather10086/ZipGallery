package archive

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type Rename struct {
	Before string `json:"before"`
	After  string `json:"after"`
}
type RepairPreview struct {
	Changes []Rename `json:"changes"`
	Total   int      `json:"total"`
	Version string   `json:"version"`
}

func legacyEncoding(name string) (encoding.Encoding, error) {
	switch name {
	case "gb18030":
		return simplifiedchinese.GB18030, nil
	case "shift-jis":
		return japanese.ShiftJIS, nil
	case "cp437":
		return charmap.CodePage437, nil
	}
	return nil, fmt.Errorf("E_ENCODING")
}
func renameLegacy(s string, enc encoding.Encoding) (string, error) {
	if utf8.ValidString(s) {
		return s, nil
	}
	b, err := enc.NewDecoder().Bytes([]byte(s))
	if err != nil || strings.ContainsRune(string(b), utf8.RuneError) {
		return "", fmt.Errorf("E_DECODE_NAME")
	}
	return string(b), nil
}
func planRepair(z *zip.ReadCloser, codec string) (RepairPreview, []zip.FileHeader, error) {
	enc, err := legacyEncoding(codec)
	if err != nil {
		return RepairPreview{}, nil, err
	}
	p := RepairPreview{Total: len(z.File), Changes: []Rename{}}
	headers := make([]zip.FileHeader, len(z.File))
	names := map[string]string{}
	for i, f := range z.File {
		h := f.FileHeader
		h.Name, err = renameLegacy(f.Name, enc)
		if err != nil {
			return p, nil, err
		}
		h.Comment, err = renameLegacy(f.Comment, enc)
		if err != nil {
			return p, nil, err
		}
		if previous, ok := names[h.Name]; ok && previous != f.Name {
			return p, nil, fmt.Errorf("E_NAME_COLLISION")
		}
		names[h.Name] = f.Name
		if h.Name != f.Name || f.NonUTF8 {
			p.Changes = append(p.Changes, Rename{fmt.Sprintf("%q", f.Name), h.Name})
		}
		h.NonUTF8 = false
		h.Flags |= 0x800
		h.Extra = stripUnicodeExtra(h.Extra)
		headers[i] = h
	}
	return p, headers, nil
}
func stripUnicodeExtra(extra []byte) []byte {
	var out []byte
	for len(extra) >= 4 {
		tag, n := binary.LittleEndian.Uint16(extra), int(binary.LittleEndian.Uint16(extra[2:]))
		if n+4 > len(extra) {
			break
		}
		if tag != 0x7075 && tag != 0x6375 {
			out = append(out, extra[:n+4]...)
		}
		extra = extra[n+4:]
	}
	return out
}
func PreviewRepair(path, codec string) (RepairPreview, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return RepairPreview{}, fmt.Errorf("E_BAD_ZIP: %w", err)
	}
	defer z.Close()
	p, _, err := planRepair(z, codec)
	st, e := os.Stat(path)
	if e != nil {
		return p, e
	}
	p.Version = fmt.Sprintf("%d-%d", st.Size(), st.ModTime().UnixNano())
	return p, err
}

// RepairTo always creates a new file; never replaces the source or existing data.
func RepairTo(source, destination, codec, version string) error {
	a, _ := filepath.Abs(source)
	b, _ := filepath.Abs(destination)
	if a == b {
		return fmt.Errorf("E_SAME_FILE")
	}
	st, err := os.Stat(source)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%d-%d", st.Size(), st.ModTime().UnixNano()) != version {
		return fmt.Errorf("E_CHANGED")
	}
	z, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer z.Close()
	_, headers, err := planRepair(z, codec)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("E_NEW_FILE: %w", err)
	}
	success := false
	defer func() {
		out.Close()
		if !success {
			os.Remove(destination)
		}
	}()
	w := zip.NewWriter(out)
	enc, _ := legacyEncoding(codec)
	comment, err := renameLegacy(z.Comment, enc)
	if err != nil {
		return err
	}
	if err = w.SetComment(comment); err != nil {
		return err
	}
	for i, f := range z.File {
		r, err := f.OpenRaw()
		if err != nil {
			return err
		}
		dst, err := w.CreateRaw(&headers[i])
		if err != nil {
			return err
		}
		if _, err = io.Copy(dst, r); err != nil {
			return err
		}
	}
	if err = w.Close(); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	success = true
	return nil
}
