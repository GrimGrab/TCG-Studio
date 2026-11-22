package unityfs

import "errors"

// The game's card frames: MonsterData_ScriptableObject.m_CardUISettingList (one CardUISetting per expansion, each a list of
// CardUISettingData = one card style for some borders). Read field by field in the order of the game's classes
// (Assembly-CSharp MonsterData_ScriptableObject, MonsterData, Stats, GradeCardServiceData, CardUISetting, CardUISettingData,
// CardUIGenericTextData); bools take 4 bytes (flag), like everywhere in the game's scripts.

// CardStyle is one CardUISettingData: the borders it is used for and the pictures that make their rim.
type CardStyle struct {
	Borders      []int32 // applicableBorderList (ECardBorderType)
	BorderImages []PPtr  // cardBorderImageList: the rim sheet per border (index = ECardBorderType, [0] when shorter)
	BorderMask   PPtr    // cardBorderMask: the rim's shape (a Mask over the sheet)
	ShowBorder   bool
}

// CardUISetting is one expansion's card styles.
type CardUISetting struct {
	Expansion int32
	Styles    []CardStyle
}

// ReadCardUISettings reads m_CardUISettingList of the game's MonsterData_ScriptableObject.
func (e *Env) ReadCardUISettings(o *Object) (out []CardUISetting, err error) {
	defer catch(&err, "MonsterData_ScriptableObject")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return nil, err
	}
	r := mb.fields
	for range 5 { // the shown monster lists
		r.i32s()
	}
	for range 4 { // the monster data lists
		n := r.count(40)
		for range n {
			skipMonsterData(r)
		}
	}
	for range r.count(16) { // m_RarityColor
		r.vec4()
	}
	r.pptrs()               // card backs
	r.pptrs()               // foil masks
	r.pptrs()               // graded card scratches
	for range r.count(12) { // m_GradeCardServiceDataList
		r.str()
		r.i32()
		r.f32()
	}
	out = make([]CardUISetting, r.count(40))
	for i := range out {
		s := &out[i]
		s.Expansion = r.i32()
		r.flag() // openPackCanUseRarity
		r.flag() // openPackCanHaveDuplicate
		r.i32()  // artistNameIndex
		r.i32()  // iconIndex
		r.i32()  // bgIndex
		r.i32()  // previousEvoStageIconExpansion
		s.Styles = make([]CardStyle, r.count(40))
		for j := range s.Styles {
			s.Styles[j] = readCardStyle(r)
		}
	}
	// The two lists after it, then the end: a check that every field above was read as the game wrote it.
	r.pptrs()               // m_TetramonImageList
	for range r.count(40) { // m_SpecialCardImageList
		skipMonsterData(r)
	}
	if r.p != len(r.b) {
		return nil, errCardLayout
	}
	return out, nil
}

func skipMonsterData(r *reader) {
	r.str()        // Name
	r.str()        // ArtistName
	strs(r)        // ArtistNameList
	r.str()        // Description
	strs(r)        // GenericTextList
	r.vec3()       // EffectAmount
	r.i32()        // ElementIndex
	r.i32()        // Rarity
	r.i32()        // MonsterType
	r.i32()        // NextEvolution
	r.i32()        // PreviousEvolution
	r.i32s()       // Roles
	r.skip(16 * 4) // BaseStats: 16 ints
	r.i32s()       // SkillList
	r.pptr()       // Icon
	r.pptr()       // GhostIcon
	r.pptrs()      // IconList
	r.pptrs()      // BGList
}

func strs(r *reader) []string {
	out := make([]string, r.count(4))
	for i := range out {
		out[i] = r.str()
	}
	return out
}

func readCardStyle(r *reader) (s CardStyle) {
	s.Borders = r.i32s()
	for range r.count(40) { // genericTextDataList
		r.flag() // showGenericText
		r.flag() // replaceMonsterNameText
		r.vec3() // PosOffset
		r.vec3() // ScaleOffset
		r.pptr() // fontMaterial
		r.i32()  // fontStyle
	}
	r.pptrs() // cardBGList
	s.BorderImages = r.pptrs()
	strs(r)   // cardBorderNameList
	strs(r)   // cardBorderFullNameList
	r.pptrs() // cardRarityImageList
	r.pptrs() // cardFrontImageList
	r.pptr()  // cardCenterFrameMask
	s.BorderMask = r.pptr()
	r.pptr()      // statImage
	for range 5 { // the BG / evo / effect / description colours
		r.vec4()
	}
	r.flag() // showCardFront
	r.flag() // showCardFrontTopLayer
	s.ShowBorder = r.flag()
	for range 16 { // showCardFullBG … showBorderFoilGlitter
		r.flag()
	}
	for range 30 { // the position / scale offsets
		r.vec3()
	}
	for range 4 { // foil materials
		r.pptrs()
	}
	return s
}

var errCardLayout = errors.New("the card data didn't read to its end (the game's format may have changed)")
