package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/pprof/profile"
	"github.com/jszwec/csvutil"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/sync/errgroup"
)

func main() {
	cmd := Cmd{}
	flag.Parse()
	cmd.InDir = flag.Arg(0)
	cmd.OutDir = flag.Arg(1)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

type Cmd struct {
	InDir  string
	OutDir string
}

func (c *Cmd) Run() error {
	files, err := c.inFiles()
	if err != nil {
		return err
	}

	var analyzers []*Analyzer
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		prof, err := profile.ParseData(data)
		if err != nil {
			return err
		}

		for _, compression := range []Compression{CompressionNone, CompressionGzip, CompressionZstd} {
			a := &Analyzer{
				Filename:    f,
				Profile:     prof,
				Compression: compression,
				OutDir:      c.OutDir,
			}
			analyzers = append(analyzers, a)
		}
	}

	eg := errgroup.Group{}
	results := make([]*Result, len(analyzers))
	for i, a := range analyzers {
		i, a := i, a
		eg.Go(func() error {
			result, err := a.Run()
			results[i] = result
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	cw := csv.NewWriter(os.Stdout)
	defer cw.Flush()
	e := csvutil.NewEncoder(cw)
	return e.Encode(results)
}

func (c *Cmd) inFiles() ([]string, error) {
	return filepath.Glob(filepath.Join(c.InDir, "*.pprof"))
}

type Analyzer struct {
	Filename    string
	Profile     *profile.Profile
	Compression Compression
	OutDir      string
}

func (a *Analyzer) Run() (*Result, error) {
	r := &Result{
		Filename:    filepath.Base(a.Filename),
		Compression: string(a.Compression),
	}

	mapping := map[Variant]*int{
		VariantPlain:     &r.PlainBytes,
		VariantBreakdown: &r.BreakdownBytes,
		VariantLabel:     &r.LabelBytes,
	}
	for variant, m := range mapping {
		variantProf := variant.Derive(a.Profile)
		data, err := a.Compression.Apply(variantProf)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(a.outFilename(variant), data, 0755); err != nil {
			return nil, err
		}
		*m = len(data)
	}
	return r, nil
}

func (a *Analyzer) outFilename(variant Variant) string {
	before, _, _ := strings.Cut(filepath.Base(a.Filename), ".pprof")
	return filepath.Join(a.OutDir, before+"."+string(a.Compression)+"."+string(variant)+".pprof")
}

type Result struct {
	Filename       string `csv:"filename"`
	Compression    string `csv:"compression"`
	PlainBytes     int    `csv:"plain (byte)"`
	LabelBytes     int    `csv:"label (byte)"`
	BreakdownBytes int    `csv:"breakdown (byte)"`
}

type Variant string

const (
	VariantPlain     = "plain"
	VariantBreakdown = "breakdown"
	VariantLabel     = "label"
)

func (v Variant) Derive(prof *profile.Profile) *profile.Profile {
	if v == VariantBreakdown {
		return prof
	}
	prof = prof.Copy()
	switch v {
	case VariantPlain:
		for _, s := range prof.Sample {
			s.Breakdown = nil
		}
		prof.LabelSet = nil
	case VariantLabel:
		var newSamples []*profile.Sample
		for _, s := range prof.Sample {
			for i := range s.Breakdown {
				b := &s.Breakdown[i]
				for i, tick := range b.Tick {
					newS := *s

					if b.LabelSet[i] != nil {
						newS.Label = b.LabelSet[i].Label
						newS.NumLabel = b.LabelSet[i].NumLabel
						newS.NumUnit = b.LabelSet[i].NumUnit
					}
					if newS.NumLabel == nil {
						newS.NumLabel = make(map[string][]int64)
					}
					newS.NumLabel["time"] = append(newS.NumLabel["time"], tick)
					newS.Breakdown = nil
					newS.Value = make([]int64, len(s.Value))
					copy(newS.Value, s.Value)
					newS.Value[0] = newS.Value[0] / int64(len(b.Tick))
					newSamples = append(newSamples, &newS)
				}
			}
		}
		prof.Sample = newSamples
		prof.LabelSet = nil
	default:
		panic(fmt.Sprintf("bug: unknown variant: %q", v))
	}
	return prof
}

type Compression string

var (
	CompressionNone Compression = "none"
	CompressionGzip Compression = "gzip"
	CompressionZstd Compression = "zstd"
)

func (c Compression) Apply(prof *profile.Profile) ([]byte, error) {
	var buf bytes.Buffer
	switch c {
	case CompressionNone:
		if err := prof.WriteUncompressed(&buf); err != nil {
			return nil, err
		}
	case CompressionGzip:
		if err := prof.Write(&buf); err != nil {
			return nil, err
		}
	case CompressionZstd:
		var uncompressedBuf bytes.Buffer
		if err := prof.WriteUncompressed(&uncompressedBuf); err != nil {
			return nil, err
		}

		enc, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(enc, &uncompressedBuf); err != nil {
			enc.Close()
			return nil, err
		} else if err := enc.Close(); err != nil {
			return nil, err
		}
	default:
		panic(fmt.Sprintf("bug: unknown compression: %q", c))
	}
	return buf.Bytes(), nil
}

type OldCmd struct {
	InFile string
	OutDir string
}

func (c *OldCmd) Run() error {
	for _, gzip := range []bool{false, true} {
		if err := c.convertToLabels(gzip); err != nil {
			return err
		} else if err := c.convertToPlain(gzip); err != nil {
			return err
		}
		units := []string{"1-nanoseconds", "2-microseconds", "3-milliseconds"}
		for _, unit := range units {
			if err := c.convertToTickUnit(unit, gzip); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *OldCmd) convertToLabels(gzip bool) error {
	prof, err := c.readProfile()
	if err != nil {
		return err
	}
	var newSamples []*profile.Sample
	for _, s := range prof.Sample {
		for i := range s.Breakdown {
			b := &s.Breakdown[i]
			for i, tick := range b.Tick {
				newS := *s

				if b.LabelSet[i] != nil {
					newS.Label = b.LabelSet[i].Label
					newS.NumLabel = b.LabelSet[i].NumLabel
					newS.NumUnit = b.LabelSet[i].NumUnit
				}
				if newS.NumLabel == nil {
					newS.NumLabel = make(map[string][]int64)
				}
				newS.NumLabel["time"] = append(newS.NumLabel["time"], tick)
				newS.Breakdown = nil
				newS.Value = make([]int64, len(s.Value))
				copy(newS.Value, s.Value)
				newS.Value[0] = newS.Value[0] / int64(len(b.Tick))
				newSamples = append(newSamples, &newS)
			}
		}
	}
	prof.Sample = newSamples
	prof.LabelSet = nil
	return c.writeProfile(prof, "labels", gzip)
}

func (c *OldCmd) convertToPlain(gzip bool) error {
	prof, err := c.readProfile()
	if err != nil {
		return err
	}
	var newSamples []*profile.Sample
	for _, s := range prof.Sample {
		newS := *s
		newS.Breakdown = nil
		newSamples = append(newSamples, &newS)
	}
	prof.Sample = newSamples
	prof.LabelSet = nil
	return c.writeProfile(prof, "plain", gzip)
}

func (c *OldCmd) convertToTickUnit(unit string, gzip bool) error {
	prof, err := c.readProfile()
	if err != nil {
		return err
	}
	for _, s := range prof.Sample {
		for i := range s.Breakdown {
			b := &s.Breakdown[i]
			for j, v := range b.Tick {
				switch unit {
				case "1-nanoseconds":
					// do nothing
				case "2-microseconds":
					v = v / 1e3
				case "3-milliseconds":
					v = v / 1e6
				default:
					panic("bug")
				}
				b.Tick[j] = v
			}

		}
	}
	prof.TickUnit = unit
	prof.LabelSet = nil
	return c.writeProfile(prof, unit, gzip)
}

func (c *OldCmd) readProfile() (*profile.Profile, error) {
	data, err := os.ReadFile(c.InFile)
	if err != nil {
		return nil, err
	}
	prof, err := profile.ParseData(data)
	if err != nil {
		return nil, err
	}
	if prof.TickUnit != "nanoseconds" {
		return nil, fmt.Errorf("unexpected tick_unit: %q", prof.TickUnit)
	}
	return prof, nil
}

func (c *OldCmd) writeProfile(prof *profile.Profile, suffix string, gzip bool) error {
	var compression string
	var buf bytes.Buffer
	if gzip {
		compression = "compressed"
		if err := prof.Write(&buf); err != nil {
			return err
		}
	} else {
		compression = "uncompressed"
		if err := prof.WriteUncompressed(&buf); err != nil {
			return err
		}
	}
	filename := c.variantFilename(compression + "." + suffix)
	return os.WriteFile(filename, buf.Bytes(), 0755)
}
func (c *OldCmd) variantFilename(suffix string) string {
	before, _, _ := strings.Cut(filepath.Base(c.InFile), ".pprof")
	return filepath.Join(c.OutDir, before+"."+suffix+".pprof")
}
