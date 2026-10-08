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
	CustomerStandLoc                                   PPtr   // Transform
	PriceTags                                          []PPtr // InteractablePriceTag components
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
	c.CustomerStandLoc = r.pptr()
	r.pptr() // stored item list grp
	r.pptr() // gamepad aim loc
	c.PriceTags = r.pptrs()
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
	readInteractableObject(r)
	s.ItemNotForSale = r.flag()
	r.flag() // gamepad quick select reverse
	s.CompartmentGroups = r.pptrs()
	return s, nil
}

// InteractableObject: the base fields every furniture piece starts with.
type InteractableObject struct {
	GameObject                         PPtr
	ObjectType                         int32
	Highlight, NavMeshCut              PPtr // GameObjects (helpers, not part of the look)
	Mesh, CullingMesh                  PPtr // MeshRenderers
	IsGeneric                          bool
	IsDecorationVertical               bool // decorations: hangs on a wall (else stands on the floor)
	PickupMesh, ValidArea, BoxCollider PPtr // MeshFilter, Transform (placement area), BoxCollider
}

func readInteractableObject(r *reader) (io InteractableObject) {
	io.ObjectType = r.i32()
	r.i32() // deco object type
	io.Highlight = r.pptr()
	io.NavMeshCut = r.pptr()
	io.Mesh = r.pptr()
	io.CullingMesh = r.pptr()
	r.pptr() // skin mesh
	r.f32()  // highlight outline width
	io.IsGeneric = r.flag()
	// can pickup move, can box up, can scan by counter, place in shop only, place in warehouse only, allow place near shop,
	// is decoration vertical, can flip, can cat stand on this
	for i := 0; i < 9; i++ {
		v := r.flag()
		if i == 6 {
			io.IsDecorationVertical = v
		}
	}
	io.PickupMesh = r.pptr()
	io.ValidArea = r.pptr()
	r.pptr() // shelf valid area
	io.BoxCollider = r.pptr()
	r.pptr()  // cat stand collider
	r.pptrs() // box collider list
	r.i32s()  // game action input display list
	r.i32s()  // controller-only list
	return io
}

// ReadInteractableObject decodes the base fields of any InteractableObject script (subclass fields follow, unread).
func ReadInteractableObject(o *Object) (io InteractableObject, err error) {
	defer catch(&err, "InteractableObject")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return io, err
	}
	io = readInteractableObject(mb.fields)
	io.GameObject = mb.GameObject
	return io, nil
}

// SpotGroups: a furniture script's compartment groups (Transforms whose children are the compartments) — item compartments
// (ShelfCompartment; warehouse ones also carry InteractableStorageCompartment) and card compartments.
type SpotGroups struct {
	Items, Cards []PPtr
}

// ReadSpotGroups reads the compartment groups of Shelf, WarehouseShelf, CardShelf, CardItemCombiShelf and TournamentPrizeShelf
// (other classes have none). Mirrors the mod's FurnitureKinds.ItemGroups/CardGroups.
func ReadSpotGroups(o *Object, class string) (g SpotGroups, err error) {
	defer catch(&err, class)
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return g, err
	}
	r := mb.fields
	readInteractableObject(r)
	switch class {
	case "Shelf":
		r.flag() // item not for sale
		r.flag() // gamepad quick select reverse
		g.Items = r.pptrs()
	case "WarehouseShelf":
		g.Items = r.pptrs()
	case "CardShelf", "CardItemCombiShelf", "TournamentPrizeShelf":
		r.flag() // item not for sale
		g.Cards = r.pptrs()
		if class != "CardShelf" {
			r.pptr() // electronic card listener
			r.flag() // gamepad quick select reverse
			g.Items = r.pptrs()
		}
	}
	return g, nil
}

// CardCompartment is InteractableCardCompartment (one card spot).
type CardCompartment struct {
	GameObject                           PPtr
	CustomerStandLoc, PutCardLoc, AimLoc PPtr // Transforms
	PriceTags                            []PPtr
}

func ReadCardCompartment(o *Object) (c CardCompartment, err error) {
	defer catch(&err, "InteractableCardCompartment")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return c, err
	}
	r := mb.fields
	c.GameObject = mb.GameObject
	r.flag() // item not for sale
	r.flag() // hide adapter mesh
	r.flag() // none-graded card uses alt location
	c.CustomerStandLoc = r.pptr()
	r.pptr() // stored item list grp
	c.PutCardLoc = r.pptr()
	r.pptr() // put card location alt
	c.AimLoc = r.pptr()
	r.pptr() // adapter mesh
	c.PriceTags = r.pptrs()
	return c, nil
}

// ObjectData / FurniturePurchaseData (ShelfData_ScriptableObject).
type ObjectData struct {
	Name       string // I2 term
	ObjectType int32
	Prefab     PPtr // InteractableObject script
	DecoBonus  float32
}

type FurniturePurchase struct {
	Name, Description string // I2 terms
	Level             int32
	Price             float32
	ObjectType        int32
	Icon              PPtr
}

// DecoData / DecoPurchaseData: one placeable decoration (posters, statues, signs, plants, fan art), indexed by EDecoObject.
type DecoData struct {
	Name     string // I2 term
	DecoType int32  // EDecoType: None, Poster, Statue, Sign, Plants
	Prefab   PPtr   // InteractableObject script
}

type DecoPurchase struct {
	Name, MainName, ReplaceXXX, ReplaceYYY string // I2 terms (MainName "… XXX …" with the others filled in)
	Level                                  int32  // unused by the game
	Price                                  float32
	Icon                                   PPtr
}

// ShopDeco is one wall / wall bar / floor / ceiling look (ShopDecoData), picked by its list index.
type ShopDeco struct {
	Name, MainName, ReplaceXXX, ReplaceYYY string
	Price                                  float32
	ShowBar                                bool
	MainTexture, RoughnessMap, NormalMap   PPtr
	Color                                  [4]float32
	Smoothness                             float32
	Icon                                   PPtr
}

type ShelfData struct {
	File      *File
	Objects   []ObjectData
	Purchases []FurniturePurchase
	// Decorations (after the furniture lists).
	Decos                         []DecoData
	DecoPurchases                 []DecoPurchase
	Floors, Walls, WallBars, Ceil []ShopDeco
	PosterList, OtherList         []int32 // EDecoObject values shown on the shop's Poster / Other tabs
}

func readShopDecos(r *reader) []ShopDeco {
	out := make([]ShopDeco, r.count(48))
	for i := range out {
		d := &out[i]
		d.Name, d.MainName, d.ReplaceXXX, d.ReplaceYYY = r.str(), r.str(), r.str(), r.str()
		d.Price = r.f32()
		d.ShowBar = r.flag()
		d.MainTexture, d.RoughnessMap, d.NormalMap = r.pptr(), r.pptr(), r.pptr()
		d.Color = r.vec4()
		d.Smoothness = r.f32()
		d.Icon = r.pptr()
	}
	return out
}

// ReadShelfData decodes ShelfData_ScriptableObject: furniture and decoration lists.
func ReadShelfData(o *Object) (s ShelfData, err error) {
	defer catch(&err, "ShelfData_ScriptableObject")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return s, err
	}
	r := mb.fields
	s.File = o.File
	s.Objects = make([]ObjectData, r.count(24))
	for i := range s.Objects {
		d := &s.Objects[i]
		d.Name = r.str()
		d.ObjectType = r.i32()
		d.Prefab = r.pptr()
		d.DecoBonus = r.f32()
	}
	s.Purchases = make([]FurniturePurchase, r.count(32))
	for i := range s.Purchases {
		d := &s.Purchases[i]
		d.Name = r.str()
		d.Description = r.str()
		d.Level = r.i32()
		d.Price = r.f32()
		d.ObjectType = r.i32()
		d.Icon = r.pptr()
	}
	s.Decos = make([]DecoData, r.count(20))
	for i := range s.Decos {
		d := &s.Decos[i]
		d.Name = r.str()
		d.DecoType = r.i32()
		d.Prefab = r.pptr()
	}
	s.DecoPurchases = make([]DecoPurchase, r.count(36))
	for i := range s.DecoPurchases {
		d := &s.DecoPurchases[i]
		d.Name, d.MainName, d.ReplaceXXX, d.ReplaceYYY = r.str(), r.str(), r.str(), r.str()
		d.Level = r.i32()
		d.Price = r.f32()
		d.Icon = r.pptr()
	}
	s.Floors = readShopDecos(r)
	s.Walls = readShopDecos(r)
	s.WallBars = readShopDecos(r)
	s.Ceil = readShopDecos(r)
	s.PosterList = r.i32s()
	s.OtherList = r.i32s()
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

// ReadUIPrefabs reads the world-UI prefabs the furniture screens are made from: WorldCanvasUIManager (till screen, card screen) or
// AutoCardOpenerUISpawner (pack opener progress screen); the first fields of either script.
func ReadUIPrefabs(o *Object, n int) (out []PPtr, err error) {
	defer catch(&err, "UI prefabs")
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return nil, err
	}
	for i := 0; i < n; i++ {
		out = append(out, mb.fields.pptr())
	}
	return out, nil
}

// ComponentGameObject is the GameObject a component (script, Animation, renderer…) sits on: every component's first field.
func ComponentGameObject(o *Object) (p PPtr, err error) {
	defer catch(&err, "Component")
	r, err := o.reader()
	if err != nil {
		return p, err
	}
	return r.pptr(), nil
}

// PointRef is a furniture point's object: a Transform, a GameObject or a component (its GameObject's Transform is the point).
type PointRef struct {
	Ptr PPtr
	Of  RefKind
}

type RefKind int

const (
	RefTransform RefKind = iota
	RefGameObject
	RefComponent
)

// ReadFurniturePoints reads the points of a furniture script (the fields after InteractableObject's) by the mod's role names
// (Core/FurnitureKinds.cs PointRoles, setfmt.PointRoles). Classes without points return an empty map.
func ReadFurniturePoints(o *Object, class string) (roles map[string][]PointRef, err error) {
	defer catch(&err, class)
	roles = map[string][]PointRef{}
	mb, err := readMonoBehaviour(o)
	if err != nil {
		return roles, err
	}
	r := mb.fields
	readInteractableObject(r)
	ref := func(role string, of RefKind) { roles[role] = append(roles[role], PointRef{r.pptr(), of}) }
	one := func(role string) { ref(role, RefTransform) }
	list := func(role string) {
		for _, p := range r.pptrs() {
			roles[role] = append(roles[role], PointRef{p, RefTransform})
		}
	}
	switch class {
	case "InteractablePlayTable":
		list("stand")
		list("standB")
		list("sit")
	case "InteractableCashierCounter":
		one("cashier")
		one("queue")
		one("placeItems")
		one("scanItem")
		one("money")
		one("coin")
		one("screen")
		one("cardScreen") // not a role: on the card machine, moves with it (its screen preview's place)
		ref("cardMachine", RefTransform)
		one("cardPay")
		one("cardLook")
		list("trade")
		r.pptr() // credit card model
		ref("bag", RefGameObject)
		r.pptr() // nav-mesh cut when manned
		ref("closedSign", RefGameObject)
		ref("tradeSign", RefGameObject)
		ref("drawer", RefComponent) // the drawer's Animation (the role moves its parent)
	case "InteractableAutoPackOpener":
		one("packIn")
		one("packInside")
		one("screen")
		one("worker")
	case "InteractableAutoCleanser":
		r.pptrs() // item pos list
		one("worker")
	case "InteractableWorkbench":
		one("player")
	case "InteractableBulkDonationBox", "InteractableCardStorageShelf":
		list("customer")
	case "InteractableEmptyBoxStorage":
		r.pptr() // box stack
		r.pptr() // box spawn loc
		list("customer")
	case "TournamentPrizeShelf": // after CardShelf's and CardItemCombiShelf's fields
		r.flag()  // item not for sale
		r.pptrs() // card compartment groups
		r.pptr()  // electronic card listener
		r.flag()  // gamepad quick select reverse
		r.pptrs() // item compartment groups
		list("customer")
		list("winner")
		ref("screen", RefGameObject)
	}
	return roles, nil
}
