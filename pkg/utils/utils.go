package utils

import "math/big"

func InStrSlice(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func GetConfirmations(current, block *big.Int) *big.Int {
	confirmations := big.NewInt(0).Sub(current, block)
	confirmations = confirmations.Add(confirmations, big.NewInt(1))
	return confirmations
}
