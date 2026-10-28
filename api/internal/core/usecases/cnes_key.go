package usecases

import "strings"

// The CNES id a lead already carries.
//
// A lead the park hunt created has a contact keyed "cnes:5558638" — the
// establishment id, in the identity field, put there by the code that created
// it. Both establishment matchers ignored it and guessed instead: phone
// digits, then a name search that only counts when it returns exactly one hit.
//
// For a hunted lead neither guess lands. The registry phone often fails E.164
// normalisation (that is WHY the contact stayed keyed by cnes), and hospital
// names repeat across a state. So the answer sat in the key while three
// features went quiet at once: the park on the lead panel, the park in the
// DOSSIER — so the draft did not know what the hospital owns — and the
// incumbent supplier, which needs the establishment to reach its CNPJ.
const cnesContactKeyPrefix = "cnes:"

// cnesIDFromKey returns the establishment id carried by a contact key, or "".
func cnesIDFromKey(key string) string {
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(strings.ToLower(key), cnesContactKeyPrefix) {
		return ""
	}
	id := strings.TrimSpace(key[len(cnesContactKeyPrefix):])
	// CNES ids are seven digits. Anything else is not one, and guessing from a
	// malformed key is how a lead ends up showing another hospital's park.
	if len(id) != 7 {
		return ""
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return id
}
