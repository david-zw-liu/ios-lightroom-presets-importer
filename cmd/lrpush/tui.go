package main

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// tuiMultiSelect shows an arrow-key multi-select (↑/↓ move, space toggle, enter
// submit) over the given entries, followed by a yes/no confirm. It returns the
// selected entry names and confirmed=true only when the user submits a non-empty
// selection and confirms. A user abort (esc/ctrl-c) is treated as a cancel, not
// an error.
func tuiMultiSelect(entries []entryChoice) (selected []string, confirmed bool, err error) {
	opts := make([]huh.Option[string], len(entries))
	for i, e := range entries {
		kind := "file"
		if e.IsDir {
			kind = "dir "
		}
		opts[i] = huh.NewOption(fmt.Sprintf("%s  %s", kind, e.Name), e.Name)
	}

	if err := huh.NewMultiSelect[string]().
		Title("Select userStyles items to delete").
		Description("↑/↓ move · space toggle · enter submit · esc cancel").
		Options(opts...).
		Value(&selected).
		Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(selected) == 0 {
		return nil, false, nil
	}

	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Delete %d selected item(s)? (backs up first)", len(selected))).
		Affirmative("Delete").
		Negative("Cancel").
		Value(&confirmed).
		Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return selected, confirmed, nil
}
