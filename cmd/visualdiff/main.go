// Command visualdiff compares PNG images or PNG sequences pixel-for-pixel and
// reports the first difference, exiting non-zero on any mismatch or error. It
// is a thin CLI over github.com/trancecode/vantage/visualtest.
//
// Usage:
//
//	visualdiff <golden> <candidate>
//
// When both arguments are directories, their .png files are compared as ordered
// sequences (sorted by name). When both are files, the two images are compared
// directly.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/trancecode/vantage/visualtest"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: visualdiff <golden> <candidate>")
		fmt.Fprintln(os.Stderr, "  compares two PNG files, or two directories of PNG frames, pixel-for-pixel")
	}
	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(flag.Arg(0), flag.Arg(1)); err != nil {
		fmt.Fprintln(os.Stderr, "visualdiff:", err)
		os.Exit(1)
	}
}

// run compares golden against candidate, diffing them as sequences when both
// are directories and as single images otherwise. It returns a non-nil error
// on any mismatch (the mismatch itself is an error) or I/O failure.
func run(golden, candidate string) error {
	goldenDir, err := isDir(golden)
	if err != nil {
		return err
	}
	candidateDir, err := isDir(candidate)
	if err != nil {
		return err
	}

	if goldenDir && candidateDir {
		return runSequences(golden, candidate)
	}
	if goldenDir != candidateDir {
		return fmt.Errorf("%q and %q must both be files or both be directories", golden, candidate)
	}

	mismatch, err := visualtest.ComparePNGFiles(golden, candidate)
	if err != nil {
		return err
	}
	if mismatch != nil {
		return mismatch
	}
	fmt.Println("match: images identical")
	return nil
}

// runSequences compares the PNG sequences in the golden and candidate
// directories.
func runSequences(golden, candidate string) error {
	wantPaths, err := visualtest.PNGSequence(golden)
	if err != nil {
		return fmt.Errorf("listing golden frames: %w", err)
	}
	gotPaths, err := visualtest.PNGSequence(candidate)
	if err != nil {
		return fmt.Errorf("listing candidate frames: %w", err)
	}

	mismatch, err := visualtest.CompareSequences(wantPaths, gotPaths)
	if err != nil {
		return err
	}
	if mismatch != nil {
		return mismatch
	}
	fmt.Printf("match: %d frames identical\n", len(wantPaths))
	return nil
}

// isDir reports whether path is a directory.
func isDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err // os.PathError already includes operation and filename
	}
	return info.IsDir(), nil
}
