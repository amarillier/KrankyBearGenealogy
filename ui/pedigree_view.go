package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"genealogy/store"
)

// PedigreeView displays an ancestor chart in PAF style.
type PedigreeView struct {
	widget.BaseWidget
	store         *store.Store
	currentPerson *store.Person
	window        fyne.Window
	onNavigate    func(personID int64)
	onEdit        func(personID int64)
	content       *fyne.Container
}

// NewPedigreeView creates a new pedigree chart widget.
func NewPedigreeView(s *store.Store, w fyne.Window, onNavigate func(personID int64), onEdit func(personID int64)) *PedigreeView {
	pv := &PedigreeView{
		store:      s,
		window:     w,
		onNavigate: onNavigate,
		onEdit:     onEdit,
	}
	pv.ExtendBaseWidget(pv)
	return pv
}

// SetPerson updates the view to show a specific person's pedigree.
func (pv *PedigreeView) SetPerson(p *store.Person) {
	pv.currentPerson = p
	pv.refresh()
	pv.Refresh() // Force widget refresh
}

// CreateRenderer implements the widget interface.
func (pv *PedigreeView) CreateRenderer() fyne.WidgetRenderer {
	pv.content = container.NewMax()
	pv.refresh()
	return widget.NewSimpleRenderer(pv.content)
}

func (pv *PedigreeView) refresh() {
	if pv.content == nil {
		return
	}
	pv.content.Objects = nil

	if pv.currentPerson == nil {
		pv.content.Objects = []fyne.CanvasObject{widget.NewLabel("No person selected")}
		pv.content.Refresh()
		return
	}

	// Build pedigree chart showing 4 generations
	chart := pv.buildPedigreeChart()
	scroll := container.NewScroll(chart)
	
	// Replace content with scroll container
	pv.content.Objects = []fyne.CanvasObject{scroll}
	pv.content.Refresh()
}

// buildPedigreeChart creates the pedigree chart layout.
func (pv *PedigreeView) buildPedigreeChart() fyne.CanvasObject {
	// Get ancestors
	person := pv.currentPerson
	parents := pv.getParents(person.ID)
	
	var father, mother *store.Person
	if len(parents) > 0 {
		// Determine father/mother by gender
		for i := range parents {
			if parents[i].Gender == "M" {
				father = &parents[i]
			} else {
				mother = &parents[i]
			}
		}
	}

	// Generation labels
	gen1Label := widget.NewLabel("1st")
	gen2Label := widget.NewLabel("2nd")
	gen3Label := widget.NewLabel("3rd")
	gen4Label := widget.NewLabel("4th")

	// Current person (1st generation)
	personBox := pv.makePersonBox(person, true)

	// 2nd generation (parents)
	var fatherBox, motherBox fyne.CanvasObject
	if father != nil {
		fatherBox = pv.makePersonBox(father, false)
	} else {
		fatherBox = pv.makeEmptyBox()
	}
	if mother != nil {
		motherBox = pv.makePersonBox(mother, false)
	} else {
		motherBox = pv.makeEmptyBox()
	}

	// 3rd generation (grandparents)
	var ff, fm, mf, mm *store.Person
	var ffBox, fmBox, mfBox, mmBox fyne.CanvasObject
	
	if father != nil {
		fParents := pv.getParents(father.ID)
		for i := range fParents {
			if fParents[i].Gender == "M" {
				ff = &fParents[i]
			} else {
				fm = &fParents[i]
			}
		}
		if ff != nil {
			ffBox = pv.makePersonBox(ff, false)
		} else {
			ffBox = pv.makeEmptyBox()
		}
		if fm != nil {
			fmBox = pv.makePersonBox(fm, false)
		} else {
			fmBox = pv.makeEmptyBox()
		}
	} else {
		ffBox = pv.makeEmptyBox()
		fmBox = pv.makeEmptyBox()
	}

	if mother != nil {
		mParents := pv.getParents(mother.ID)
		for i := range mParents {
			if mParents[i].Gender == "M" {
				mf = &mParents[i]
			} else {
				mm = &mParents[i]
			}
		}
		if mf != nil {
			mfBox = pv.makePersonBox(mf, false)
		} else {
			mfBox = pv.makeEmptyBox()
		}
		if mm != nil {
			mmBox = pv.makePersonBox(mm, false)
		} else {
			mmBox = pv.makeEmptyBox()
		}
	} else {
		mfBox = pv.makeEmptyBox()
		mmBox = pv.makeEmptyBox()
	}

	// 4th generation (great-grandparents) - fetch and display
	var fffBox, ffmBox, fmfBox, fmmBox, mffBox, mfmBox, mmfBox, mmmBox fyne.CanvasObject
	
	// Father's father's parents
	if ff != nil {
		ffParents := pv.getParents(ff.ID)
		var fff, ffm *store.Person
		for i := range ffParents {
			if ffParents[i].Gender == "M" {
				fff = &ffParents[i]
			} else {
				ffm = &ffParents[i]
			}
		}
		fffBox = pv.makePersonBoxIfExists(fff)
		ffmBox = pv.makePersonBoxIfExists(ffm)
	} else {
		fffBox = pv.makeEmptyBox()
		ffmBox = pv.makeEmptyBox()
	}
	
	// Father's mother's parents
	if fm != nil {
		fmParents := pv.getParents(fm.ID)
		var fmf, fmm *store.Person
		for i := range fmParents {
			if fmParents[i].Gender == "M" {
				fmf = &fmParents[i]
			} else {
				fmm = &fmParents[i]
			}
		}
		fmfBox = pv.makePersonBoxIfExists(fmf)
		fmmBox = pv.makePersonBoxIfExists(fmm)
	} else {
		fmfBox = pv.makeEmptyBox()
		fmmBox = pv.makeEmptyBox()
	}
	
	// Mother's father's parents
	if mf != nil {
		mfParents := pv.getParents(mf.ID)
		var mff, mfm *store.Person
		for i := range mfParents {
			if mfParents[i].Gender == "M" {
				mff = &mfParents[i]
			} else {
				mfm = &mfParents[i]
			}
		}
		mffBox = pv.makePersonBoxIfExists(mff)
		mfmBox = pv.makePersonBoxIfExists(mfm)
	} else {
		mffBox = pv.makeEmptyBox()
		mfmBox = pv.makeEmptyBox()
	}
	
	// Mother's mother's parents
	if mm != nil {
		mmParents := pv.getParents(mm.ID)
		var mmf, mmm *store.Person
		for i := range mmParents {
			if mmParents[i].Gender == "M" {
				mmf = &mmParents[i]
			} else {
				mmm = &mmParents[i]
			}
		}
		mmfBox = pv.makePersonBoxIfExists(mmf)
		mmmBox = pv.makePersonBoxIfExists(mmm)
	} else {
		mmfBox = pv.makeEmptyBox()
		mmmBox = pv.makeEmptyBox()
	}

	// Layout in columns by generation
	gen1Col := container.NewVBox(gen1Label, widget.NewLabel(""), personBox)
	gen2Col := container.NewVBox(gen2Label, widget.NewLabel("Father:"), fatherBox, widget.NewLabel(""), widget.NewLabel("Mother:"), motherBox)
	gen3Col := container.NewVBox(gen3Label, ffBox, fmBox, widget.NewLabel(""), mfBox, mmBox)
	gen4Col := container.NewVBox(gen4Label, fffBox, ffmBox, fmfBox, fmmBox, widget.NewLabel(""), mffBox, mfmBox, mmfBox, mmmBox)

	chart := container.NewHBox(gen1Col, gen2Col, gen3Col, gen4Col)
	
	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Pedigree Chart for %s %s", person.GivenName, person.Surname),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	return container.NewBorder(title, nil, nil, nil, chart)
}

// makePersonBox creates a clickable box for a person.
func (pv *PedigreeView) makePersonBox(p *store.Person, isCurrent bool) fyne.CanvasObject {
	nameText := fmt.Sprintf("%s %s", p.GivenName, p.Surname)
	
	var dateText string
	if p.BirthDate != "" {
		dateText = "b. " + p.BirthDate
	}
	if p.DeathDate != "" {
		if dateText != "" {
			dateText += " "
		}
		dateText += "d. " + p.DeathDate
	}

	nameLabel := widget.NewLabel(nameText)
	nameLabel.Wrapping = fyne.TextWrapOff
	if isCurrent {
		nameLabel.TextStyle.Bold = true
	}

	dateLabel := widget.NewLabel(dateText)
	dateLabel.TextStyle.Italic = true
	dateLabel.Wrapping = fyne.TextWrapOff

	content := container.NewVBox(nameLabel, dateLabel)
	content.Resize(fyne.NewSize(200, 80))

	// Create a button for better interaction
	btn := widget.NewButton("", func() {
		if pv.onNavigate != nil {
			pv.onNavigate(p.ID)
		}
	})
	
	// Replace button content with our custom content
	boxed := container.NewStack(btn, container.NewPadded(content))
	boxed.Resize(fyne.NewSize(200, 80))

	return boxed
}

// makeEmptyBox creates an empty placeholder box.
func (pv *PedigreeView) makeEmptyBox() fyne.CanvasObject {
	label := widget.NewLabel("Unknown")
	label.Alignment = fyne.TextAlignCenter
	bg := canvas.NewRectangle(theme.DisabledColor())
	box := container.NewMax(bg, container.NewPadded(label))
	box.Resize(fyne.NewSize(200, 100))
	return box
}

// makePersonBoxIfExists creates a box if person exists, otherwise empty box.
func (pv *PedigreeView) makePersonBoxIfExists(p *store.Person) fyne.CanvasObject {
	if p == nil {
		return pv.makeEmptyBox()
	}
	return pv.makePersonBox(p, false)
}

// getParents returns the parents of a person.
func (pv *PedigreeView) getParents(personID int64) []store.Person {
	parents, _ := pv.store.GetRelatedPeople(personID, "parent")
	return parents
}

// tappableContainer wraps a container to handle single/double taps.
type tappableContainer struct {
	widget.BaseWidget
	content      fyne.CanvasObject
	onTap        func(doubleTap bool)
	lastTapTime  int64
	tapThreshold int64 // milliseconds
}

func newTappableContainer(content fyne.CanvasObject, onTap func(doubleTap bool)) *tappableContainer {
	t := &tappableContainer{
		content:      content,
		onTap:        onTap,
		tapThreshold: 300, // 300ms for double-tap
	}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

func (t *tappableContainer) Tapped(e *fyne.PointEvent) {
	// Simple single tap for now - double tap detection is complex in Fyne
	// User can use the "Go to Family View" button for navigation
	if t.onTap != nil {
		t.onTap(false) // Always single tap
	}
}

func (t *tappableContainer) DoubleTapped(e *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap(true)
	}
}
