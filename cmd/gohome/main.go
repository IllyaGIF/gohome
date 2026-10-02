package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"gohome/internal/build"
	"gohome/internal/compiler"
	"gohome/internal/project"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gohome:", err)
		os.Exit(1)
	}
}

func help() {
	fmt.Print(`gohome — Go source to ARMv7 packages for jailbroken iOS 6.1.3

  gohome new NAME                  Create a Cydia tweak
  gohome new NAME -kind app        Create a UIKit application
  gohome build [DIR|FILE.go]       Build DEB for a tweak, IPA for an app
  gohome build -format deb         Package an application for Cydia
  gohome check [DIR|FILE.go]       Validate the supported Go subset
  gohome doc [PACKAGE]             Show the packages and declarations available
  gohome doctor                    Compile, link and sign an ARMv7 probe
  gohome version

Build flags:
  -kind app|tweak  Override kind, useful for a single Go file
  -filter IDS      Comma-separated tweak target bundle identifiers
  -format deb|ipa   Output package type
  -o PATH          Output package path
  -sdk PATH        iPhoneOS6.1.sdk directory (or GOHOME_SDK)
  -toolchain PATH  clang/ld/ldid directory (or GOHOME_TOOLCHAIN)
  -emit            Generate native source without a toolchain
  -unsigned        Skip ad-hoc signing

The legacy backend supports a documented Go subset, gohome/ios and the
fmt, math, math/rand, os, runtime and time packages. It does not run the
official Go runtime or support arbitrary Go modules. Those packages are
declaration only stubs with Objective-C implementations, so a few functions
differ from the standard library: os has no error values and exchanges file
contents as strings, time is UTC only with int64 nanosecond timestamps, and
math/rand is a deterministic generator without goroutines.
`)
}

func parse(fs *flag.FlagSet, args []string) error {
	var flags, positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
		option := fs.Lookup(name)
		if option != nil && !strings.Contains(arg, "=") {
			boolean, ok := option.Value.(interface{ IsBoolFlag() bool })
			if !ok || !boolean.IsBoolFlag() {
				if i+1 >= len(args) {
					return fmt.Errorf("flag needs a value: %s", arg)
				}
				i++
				flags = append(flags, args[i])
			}
		}
	}
	return fs.Parse(append(flags, positionals...))
}

func run(args []string) error {
	if len(args) == 0 {
		help()
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		help()
		return nil
	case "version", "--version":
		fmt.Println("gohome", version)
		return nil
	case "new":
		fs := flag.NewFlagSet("new", flag.ContinueOnError)
		kind := fs.String("kind", "tweak", "tweak or app")
		if err := parse(fs, args[1:]); err != nil {
			if err == flag.ErrHelp {
				return nil
			}
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: gohome new NAME [-kind app|tweak]")
		}
		if err := project.Create(fs.Arg(0), *kind, compiler.SDKSource); err != nil {
			return err
		}
		fmt.Printf("Created %s (%s). Edit main.go, then run: gohome build %s\n", fs.Arg(0), *kind, fs.Arg(0))
		return nil
	case "build", "check":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		var options build.Options
		kind := fs.String("kind", "", "override project kind: app or tweak")
		filter := fs.String("filter", "", "comma-separated target bundle identifiers")
		fs.StringVar(&options.Format, "format", "", "deb or ipa")
		fs.StringVar(&options.Output, "o", "", "output path")
		fs.StringVar(&options.SDK, "sdk", "", "SDK path")
		fs.StringVar(&options.Toolchain, "toolchain", "", "toolchain path")
		fs.BoolVar(&options.EmitOnly, "emit", false, "emit native source only")
		fs.BoolVar(&options.Unsigned, "unsigned", false, "skip signing")
		if err := parse(fs, args[1:]); err != nil {
			if err == flag.ErrHelp {
				return nil
			}
			return err
		}
		if fs.NArg() > 1 {
			return fmt.Errorf("build expects one project directory or Go file")
		}
		path := "."
		if fs.NArg() == 1 {
			path = fs.Arg(0)
		}
		p, err := project.Load(path)
		if err != nil {
			return err
		}
		if *kind != "" {
			p.Config.Kind = *kind
			if *kind == "tweak" && len(p.Config.Filter) == 0 {
				p.Config.Filter = []string{"com.apple.springboard"}
			}
		}
		if *filter != "" {
			p.Config.Filter = strings.Split(*filter, ",")
			for i := range p.Config.Filter {
				p.Config.Filter[i] = strings.TrimSpace(p.Config.Filter[i])
			}
		}
		if err := p.Config.Validate(); err != nil {
			return err
		}
		if args[0] == "check" {
			r, err := compiler.Compile(p.Sources, p.Config.Kind == "app")
			if err != nil {
				return err
			}
			fmt.Printf("OK: %s, %d hook(s), iOS 6.1.3 / ARMv7\n", p.Config.Name, r.Hooks)
			return nil
		}
		r, err := build.Build(p, options)
		if err != nil {
			return err
		}
		fmt.Println(r.Output)
		if options.Unsigned && !options.EmitOnly {
			fmt.Println("Unsigned package: sign the binary before installing.")
		}
		return nil
	case "doc":
		if len(args) > 2 {
			return fmt.Errorf("doc expects at most one package name")
		}
		if len(args) == 1 {
			for _, path := range compiler.Packages() {
				fmt.Println(path)
			}
			return nil
		}
		text, err := compiler.Doc(args[1])
		if err != nil {
			return err
		}
		fmt.Print(text)
		return nil
	case "doctor":
		fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
		sdk := fs.String("sdk", "", "SDK path")
		toolchain := fs.String("toolchain", "", "toolchain directory")
		if err := parse(fs, args[1:]); err != nil {
			if err == flag.ErrHelp {
				return nil
			}
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("doctor takes only flags")
		}
		t, err := build.Discover(*sdk, *toolchain)
		if err != nil {
			return err
		}
		fmt.Printf("SDK: %s\nClang: %s\nLinker: %s\nSigner: %s\n", t.SDK, t.Clang, t.Linker, t.Signer)
		if err := t.Probe(); err != nil {
			return err
		}
		fmt.Println("OK: ARMv7 Mach-O, minimum iOS 6.1.3, ad-hoc signature")
		return nil
	default:
		return fmt.Errorf("unknown command %q; run gohome help", args[0])
	}
}
