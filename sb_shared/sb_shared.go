package cositegen_sb_shared

type Storyboard []Page

type Page struct {
	Name     string
	Balloons []Object
	Panels   []Object
}

type Object struct {
	SizeAndPos
	Paras []string
}

type SizeAndPos struct {
	CmW float64
	CmH float64
	CmX float64
	CmY float64
}
