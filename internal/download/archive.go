package download

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

func ExtractArchive(src, dest string) error {
	slog.Info("extract archive", "src", src, "dest", dest)
	lower := strings.ToLower(src)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(src, dest)
	case strings.HasSuffix(lower, ".gz"):
		return extractGzip(src, dest)
	case strings.HasSuffix(lower, ".bz2"):
		return extractBzip2(src, dest)
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(src, dest)
	default:
		return fmt.Errorf("unsupported archive: %s", src)
	}
}

func extractGzip(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gz.Close()
	return writeFile(dest, gz)
}

func extractBzip2(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeFile(dest, bzip2.NewReader(in))
}

func extractTarGz(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("no .mbtiles/.pmtiles in %s", src)
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if strings.HasSuffix(strings.ToLower(hdr.Name), ".mbtiles") || strings.HasSuffix(strings.ToLower(hdr.Name), ".pmtiles") {
			return writeFile(dest, tr)
		}
	}
}

func extractZip(src, dest string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name), ".mbtiles") || strings.HasSuffix(strings.ToLower(f.Name), ".pmtiles") {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			err = writeFile(dest, rc)
			rc.Close()
			return err
		}
	}
	return fmt.Errorf("no .mbtiles/.pmtiles in %s", src)
}

func writeFile(dest string, r io.Reader) error {
	tmp := dest + ".extract"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		slog.Warn("extract rename failed", "src", tmp, "dest", dest, "err", err)
		return err
	}
	slog.Info("extract renamed", "src", tmp, "dest", dest)
	return nil
}

func RelocateFile(src, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		slog.Info("tileset renamed", "src", src, "dest", dest)
		return nil
	} else {
		slog.Warn("tileset rename failed, copying", "src", src, "dest", dest, "err", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, in)
	if err != nil {
		out.Close()
		os.Remove(dest)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dest)
		return err
	}
	if err := os.Remove(src); err != nil {
		return err
	}
	slog.Info("tileset copied", "src", src, "dest", dest, "bytes", n)
	return nil
}
