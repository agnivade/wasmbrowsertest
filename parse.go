package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"strings"
)

// gentleParse takes a flag.FlagSet, calls Parse to get its flags parsed,
// and collects the arguments the FlagSet does not recognize, returning
// the collected list.
func gentleParse(flagset *flag.FlagSet, args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}
	const prefix = "flag provided but not defined: "

	r := make([]string, 0, len(args))

	flagset.Init(flagset.Name(), flag.ContinueOnError)
	w := flagset.Output()
	flagset.SetOutput(ioutil.Discard)

	// Put back the flagset's output, the flagset's Usage might be called later.
	defer flagset.SetOutput(w)

	next := args

	for len(next) > 0 {
		if !strings.HasPrefix(next[0], "-") {
			r, next = append(r, next[0]), next[1:]
			continue
		}
		if err := flagset.Parse(next); err != nil {
			if strings.HasPrefix(err.Error(), prefix) {
				pull := strings.TrimPrefix(err.Error(), prefix)
				for next[0] != pull && !(strings.HasPrefix(next[0], pull) && strings.HasPrefix(next[0], pull+"=")) {
					next = next[1:]
					if len(next) == 0 {
						panic("odd: pull not found: " + pull)
					}
				}
				r, next = append(r, next[0]), next[1:]
				continue
			}
			fmt.Fprintf(w, "%s\n", err)
			flagset.SetOutput(w)
			flagset.Usage()
			return nil, err
		}

		// Check if the call to flagset.Parse ate a "--". If so, we're done
		// and can return what's been built up on r along with the rest.
		if len(next) > len(flagset.Args()) {
			lastabsorbedpos := len(next) - len(flagset.Args()) - 1
			lastabsorbed := next[lastabsorbedpos]
			if lastabsorbed == "--" {
				r = append(r, "--") // return the "--" too.
				return append(r, flagset.Args()...), nil
			}
		}
		next = flagset.Args()
	}
	return r, nil
}
