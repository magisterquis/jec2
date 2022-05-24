package main

/*
 * tag.go
 * Tag which dynamically changes implant names
 * By J. Stuart McMurray
 * Created 20220522
 * Last Modified 20220524
 */

import "fmt"

// Tag is used to form a tag from an implant's name and a suffix.  If the
// implant changes name, Tag's String will reflect this.
type Tag struct {
	s   fmt.Stringer
	suf string
}

// String implements fmt.Stringer.  It returns the implant's name, a hyphen,
// and the tag suffix.
func (t Tag) String() string {
	if nil != t.s {
		return fmt.Sprintf("%s%s", t.s, t.suf)
	} else {
		return t.suf
	}
}

// Append returns a Tag whose String method returns the same as t's, plus the
// sufix prefixed with a hyphen.
func (t Tag) Append(f string, a ...any) Tag {
	return Tag{
		s:   t.s,
		suf: t.suf + "-" + fmt.Sprintf(f, a...),
	}
}
