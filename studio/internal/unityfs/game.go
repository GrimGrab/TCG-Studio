package unityfs

// Game script classes (TCG Card Shop Simulator 1.02), serialized field order as declared in Assembly-CSharp. Unity aligns
// to 4 bytes after every bool field of a script.

type ItemData struct {
	Name                                                   string
	Category                                               int32
	Icon                                                   PPtr
	IconScale, BaseCost                                    float32
	MarketPriceMinPercent, MarketPriceMaxPercent           float32
	BoxFollowItemPrice                                     int32
	IsNotBoosterPack, IsTallItem, IsHideUntilUnlocked      bool
	PosYOffsetInBox, ScaleOffsetInBox, ItemHandScaleOffset float32
	ItemDimension, ColliderPosOffset, ColliderScale        [3]float32
}

type ItemMeshData struct {
	Name                                             string
	Mesh, Material, MeshSecondary, MaterialSecondary PPtr
	MaterialList                                     []PPtr
}

type RestockData struct {
	Name                 string
	IsBigBox             bool
	Index, Amount, Level int32
	LicensePrice         float32
	ItemType             int32
}

type StockItemData struct {
	File                                                           *File
	ShownAll, Shown, ShownAccessory, ShownFigurine, ShownBoardGame []int32
	Items                                                          []ItemData
	Meshes                                                         []ItemMeshData
	Restock                                                        []RestockData
}

func (r *reader) flag() bool {
	v := r.bool()
	r.align()
	return v
}

// ReadStockItemData decodes StockItemData_ScriptableObject up to m_RestockDataList.
func (e *Env) ReadStockItemData(o *Object) (s StockItemData, err error) {
	defer catch(&err, "StockItemData_ScriptableObject")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return s, err
	}
	r := mb.fields
	s.File = o.File
	s.ShownAll = r.i32s()
	s.Shown = r.i32s()
	s.ShownAccessory = r.i32s()
	s.ShownFigurine = r.i32s()
	s.ShownBoardGame = r.i32s()
	r.i32s() // card pack item types
	r.i32s() // shown collection pack types
	r.i32s() // shown card expansions
	s.Items = make([]ItemData, r.count(40))
	for i := range s.Items {
		d := &s.Items[i]
		d.Name = r.str()
		d.Category = r.i32()
		d.Icon = r.pptr()
		d.IconScale = r.f32()
		d.BaseCost = r.f32()
		d.MarketPriceMinPercent = r.f32()
		d.MarketPriceMaxPercent = r.f32()
		d.BoxFollowItemPrice = r.i32()
		d.IsNotBoosterPack = r.flag()
		d.IsTallItem = r.flag()
		d.IsHideUntilUnlocked = r.flag()
		d.PosYOffsetInBox = r.f32()
		d.ScaleOffsetInBox = r.f32()
		d.ItemHandScaleOffset = r.f32()
		d.ItemDimension = r.vec3()
		d.ColliderPosOffset = r.vec3()
		d.ColliderScale = r.vec3()
		r.i32s() // affected price change types
	}
	s.Meshes = make([]ItemMeshData, r.count(56))
	for i := range s.Meshes {
		m := &s.Meshes[i]
		m.Name = r.str()
		m.Mesh = r.pptr()
		m.Material = r.pptr()
		m.MeshSecondary = r.pptr()
		m.MaterialSecondary = r.pptr()
		m.MaterialList = r.pptrs()
	}
	s.Restock = make([]RestockData, r.count(36))
	for i := range s.Restock {
		d := &s.Restock[i]
		d.Name = r.str()
		d.IsBigBox = r.flag()
		r.flag() // ignore double image
		d.Index = r.i32()
		d.Amount = r.i32()
		d.Level = r.i32()
		d.LicensePrice = r.f32()
		d.ItemType = r.i32()
		r.flag() // prologue show
		r.flag() // hide until unlocked
	}
	return s, nil
}

type ShelfCompartment struct {
	GameObject                                         PPtr
	CanPutItem, CanPutBox, ItemNotForSale              bool
	StartLoc, EndWidthLoc, EndDepthLoc, EndHeightLoc   PPtr
	PosListGrp                                         PPtr
	ApplyScaleOffset, HeightGoesUp, AffectedByTallItem bool
	SizeX, SizeY, SizeZ                                int32
}

func ReadShelfCompartment(o *Object) (c ShelfCompartment, err error) {
	defer catch(&err, "ShelfCompartment")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return c, err
	}
	r := mb.fields
	c.GameObject = mb.GameObject
	c.CanPutItem = r.flag()
	c.CanPutBox = r.flag()
	c.ItemNotForSale = r.flag()
	r.flag() // can lock item label
	c.StartLoc = r.pptr()
	c.EndWidthLoc = r.pptr()
	c.EndDepthLoc = r.pptr()
	c.EndHeightLoc = r.pptr()
	c.PosListGrp = r.pptr()
	r.pptr()  // customer stand loc
	r.pptr()  // stored item list grp
	r.pptr()  // gamepad aim loc
	r.pptrs() // price tags
	c.ApplyScaleOffset = r.flag()
	c.HeightGoesUp = r.flag()
	c.AffectedByTallItem = r.flag()
	c.SizeX = r.i32()
	c.SizeY = r.i32()
	c.SizeZ = r.i32()
	return c, nil
}

type Shelf struct {
	GameObject        PPtr
	ItemNotForSale    bool
	CompartmentGroups []PPtr // m_ShelfCompartmentGrpList (Transforms whose children are the compartments)
}

// ReadShelf decodes Shelf (and its InteractableObject base fields, skipped).
func ReadShelf(o *Object) (s Shelf, err error) {
	defer catch(&err, "Shelf")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return s, err
	}
	r := mb.fields
	s.GameObject = mb.GameObject
	r.i32() // object type
	r.i32() // deco object type
	for i := 0; i < 5; i++ {
		r.pptr() // highlight, nav mesh cut, mesh, culling mesh, skin mesh
	}
	r.f32() // highlight outline width
	for i := 0; i < 10; i++ {
		r.flag() // generic object … can cat stand on this
	}
	for i := 0; i < 5; i++ {
		r.pptr() // pickup mesh, valid area, shelf valid area, box collider, cat stand collider
	}
	r.pptrs() // box collider list
	r.i32s()  // game action input display list
	r.i32s()  // controller-only list
	s.ItemNotForSale = r.flag()
	r.flag() // gamepad quick select reverse
	s.CompartmentGroups = r.pptrs()
	return s, nil
}

// ReadTableGameItemSet returns the play table's playmat and deck box renderers (the playmat item only supplies the material).
func ReadTableGameItemSet(o *Object) (playMat, deckBox PPtr, err error) {
	defer catch(&err, "TableGameItemSet")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return PPtr{}, PPtr{}, err
	}
	r := mb.fields
	r.pptr() // deck box mesh filter
	r.pptr() // comic book mesh filter
	return r.pptr(), r.pptr(), nil
}

// ReadItemMeshFilter reads Item.m_MeshFilter (the item prefab's mesh child).
func ReadItemMeshFilter(o *Object) (gameObject, meshFilter PPtr, err error) {
	defer catch(&err, "Item")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return PPtr{}, PPtr{}, err
	}
	return mb.GameObject, mb.fields.pptr(), nil
}

// MonoBehaviourGameObject is the GameObject a script component sits on.
func MonoBehaviourGameObject(o *Object) (PPtr, error) {
	mb, err := readMonoBehaviour(o)
	return mb.GameObject, err
}
