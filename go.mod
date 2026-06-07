module github.com/caseymrm/menuet/v2

go 1.13

require github.com/caseymrm/askm v1.0.0

retract (
	v2.0.0 // unimportable — module path was missing /v2 suffix
	v2.1.0 // unimportable — module path was missing /v2 suffix
	v2.1.1 // superseded by v2.2.0 — MenuItem became an interface
)
