package unityfs

// The game's customer characters use the "CC" character customizer (Assembly-CSharp\CC): a character = a preset
// (CC_CharacterData) of hair and apparel picked by name from per-slot tables, each apparel piece with colour/print
// variants (CC_Apparel_Material_Collection).

// ApparelVariant is one CC_Apparel_Material_Collection: a label and one definition per material of the piece.
type ApparelVariant struct {
	Label string
	Defs  []ApparelMaterialDef
}

// ApparelMaterialDef is CC_Apparel_Material_Definition: shader colours _Tint, _Tint_R/G/B and the _Print texture.
type ApparelMaterialDef struct {
	MainTint, TintR, TintG, TintB [4]float32
	Print                         PPtr
}

// ApparelItem is scrObj_Apparel.Apparel.
type ApparelItem struct {
	Mesh        PPtr // prefab GameObject with the SkinnedMeshRenderers
	Name        string
	DisplayName string
	Mask        PPtr // Texture2D: hides the body under the piece
	Variants    []ApparelVariant
	Icon        PPtr
}

// ApparelTable is scrObj_Apparel: one slot's pieces (upper body, lower body, footwear, misc, hats).
type ApparelTable struct {
	Name         string
	Items        []ApparelItem
	MaskProperty string
	Label        string
}

func ReadApparelTable(o *Object) (t ApparelTable, err error) {
	defer catch(&err, "scrObj_Apparel")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return t, err
	}
	r := mb.fields
	t.Name = mb.Name
	t.Items = make([]ApparelItem, r.count(40))
	for i := range t.Items {
		it := &t.Items[i]
		it.Mesh = r.pptr()
		it.Name = r.str()
		it.DisplayName = r.str()
		r.flag() // AddCopyPoseScript
		it.Mask = r.pptr()
		r.f32() // FootOffset.HeightOffset
		r.f32() // FootRotation
		r.f32() // BallRotation
		it.Variants = make([]ApparelVariant, r.count(8))
		for j := range it.Variants {
			v := &it.Variants[j]
			v.Label = r.str()
			v.Defs = make([]ApparelMaterialDef, r.count(76))
			for k := range v.Defs {
				d := &v.Defs[k]
				d.MainTint = r.vec4()
				d.TintR = r.vec4()
				d.TintG = r.vec4()
				d.TintB = r.vec4()
				d.Print = r.pptr()
			}
		}
		it.Icon = r.pptr()
	}
	t.MaskProperty = r.str()
	t.Label = r.str()
	return t, nil
}

// CCProperty is CC_Property.
type CCProperty struct {
	Name, String  string
	Float         float32
	MaterialIndex int32
	MeshTag       string
}

func (r *reader) ccProps() []CCProperty {
	out := make([]CCProperty, r.count(20))
	for i := range out {
		out[i] = CCProperty{Name: r.str(), String: r.str(), Float: r.f32(), MaterialIndex: r.i32(), MeshTag: r.str()}
	}
	return out
}

func (r *reader) strs() []string {
	out := make([]string, r.count(4))
	for i := range out {
		out[i] = r.str()
	}
	return out
}

// CharacterPreset is CC_CharacterData.
type CharacterPreset struct {
	Name, Prefab      string
	Blendshapes       []CCProperty // face sliders: Name, Float (the game sets weight = value × 100)
	HairNames         []string     // one per hair table
	ApparelNames      []string     // one per apparel table (slot), "" = none
	ApparelMaterials  []int32      // the variant of each slot's piece
	HairColor         []CCProperty // per hair table: String = RRGGBBAA
	FloatProperties   []CCProperty
	TextureProperties []CCProperty // String = texture name (Resources.Load)
	ColorProperties   []CCProperty // String = RRGGBBAA
}

func (r *reader) characterData() (p CharacterPreset) {
	p.Name = r.str()
	p.Prefab = r.str()
	p.Blendshapes = r.ccProps()
	p.HairNames = r.strs()
	p.ApparelNames = r.strs()
	p.ApparelMaterials = r.i32s()
	p.HairColor = r.ccProps()
	p.FloatProperties = r.ccProps()
	p.TextureProperties = r.ccProps()
	p.ColorProperties = r.ccProps()
	return p
}

// HairItem is scrObj_Hair.Hairstyle.
type HairItem struct {
	Mesh      PPtr // prefab GameObject ("None" has none)
	Name      string
	ShadowMap PPtr // Texture2D painted on the head (ShadowProperty)
	Icon      PPtr
}

// HairTable is scrObj_Hair: one hair slot (hair, eyebrows, beards).
type HairTable struct {
	Name                         string
	Items                        []HairItem
	ShadowProperty, TintProperty string
}

func ReadHairTable(o *Object) (t HairTable, err error) {
	defer catch(&err, "scrObj_Hair")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return t, err
	}
	r := mb.fields
	t.Name = mb.Name
	t.Items = make([]HairItem, r.count(36))
	for i := range t.Items {
		it := &t.Items[i]
		it.Mesh = r.pptr()
		it.Name = r.str()
		it.ShadowMap = r.pptr()
		r.flag() // AddCopyPoseScript
		it.Icon = r.pptr()
	}
	t.ShadowProperty = r.str()
	t.TintProperty = r.str()
	return t, nil
}

// ReadCharacterPresets reads scrObj_Presets (Male_Presets: Male0…, Female_Presets: Female0…).
func ReadCharacterPresets(o *Object) (name string, out []CharacterPreset, err error) {
	defer catch(&err, "scrObj_Presets")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return "", nil, err
	}
	r := mb.fields
	out = make([]CharacterPreset, r.count(40))
	for i := range out {
		out[i] = r.characterData()
	}
	return mb.Name, out, nil
}

// CharacterCustomization is the component on the Male / Female customer body prefabs: its tables in slot order.
type CharacterCustomization struct {
	GameObject    PPtr
	MainMesh      PPtr // SkinnedMeshRenderer of the body
	CharacterName string
	HairTables    []PPtr
	ApparelTables []PPtr
	Presets       PPtr
}

func ReadCharacterCustomization(o *Object) (c CharacterCustomization, err error) {
	defer catch(&err, "CharacterCustomization")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return c, err
	}
	r := mb.fields
	c.GameObject = mb.GameObject
	c.MainMesh = r.pptr()
	r.pptr() // UI
	c.CharacterName = r.str()
	r.flag() // Autoload
	c.HairTables = r.pptrs()
	c.ApparelTables = r.pptrs()
	c.Presets = r.pptr()
	return c, nil
}
