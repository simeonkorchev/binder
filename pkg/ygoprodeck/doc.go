// Package ygoprodeck reads the YGOPRODeck card database over its public v7 API.
//
// # The contract in this package is UNVERIFIED
//
// Nothing here has ever been checked against the live service. The container
// this package was written in cannot reach db.ygoprodeck.com at all — the
// egress proxy answers 403 to CONNECT — so every statement below comes from
// specs/001-binder-mvp/spec.md and prior knowledge, not from a response body.
// Treat each one as an assumption a human still has to confirm; the exact curl
// commands that confirm them, and what to change when one turns out to be
// wrong, are in specs/001-binder-mvp/VERIFY-YGOPRODECK.md.
//
//	A1  The full dump is GET https://db.ygoprodeck.com/api/v7/cardinfo.php with
//	    no query parameters, and it returns every card in one response.
//	A2  The response is a JSON object with a single "data" array.
//	A3  A card carries "id" (the passcode, an integer) and "name".
//	A4  A card carries "card_sets" — objects of "set_name", "set_code",
//	    "set_rarity" — and "card_images" — objects of "id", "image_url",
//	    "image_url_small".
//	A5  The service asks for no more than about 20 requests per second.
//	A6  Images must be downloaded and self-hosted, never hotlinked.
//
// The package is shaped so that a wrong assumption is cheap to correct: every
// wire type lives in types.go and nowhere else, and FetchAllCards holds the
// only JSON decode in the package. A1 and A2 are one constant and one struct
// field; A5 is a constant and a configurable ceiling.
package ygoprodeck
