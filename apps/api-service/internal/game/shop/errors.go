package shop

import "errors"

var (
	ErrCannotBuy          = errors.New("cannot buy card: insufficient coins or team full")
	ErrCannotReroll       = errors.New("cannot reroll: insufficient coins")
	ErrCannotUpgrade      = errors.New("cannot upgrade: insufficient coins or invalid slot")
	ErrInvalidUpgradeType = errors.New("invalid upgrade type: must be 'atk' or 'hp'")
	ErrInvalidOrder       = errors.New("invalid team order")
	ErrTeamIncomplete     = errors.New("team incomplete: must have 3 cards")
	ErrNotEnoughCards     = errors.New("not enough cards in group to deal")
	ErrCardNotFound       = errors.New("card not found")
)
