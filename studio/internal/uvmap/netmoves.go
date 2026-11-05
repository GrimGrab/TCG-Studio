package uvmap

// NetVersion is the net version of layouts made now (Layout "net" field; missing = 1). Bump it with a model's NetMoves.
const NetVersion = 3

// NetMove: from layout net Version on, layers centred at net x >= From (in the older net) sit DX further right.
type NetMove struct {
	Version  int
	From, DX float64
}

// NetCopy: from layout net Version on, one face became two: layers on face From (its colours by name, the rest centred in
// Area) get a copy on face To, DX, DY away in the net.
type NetCopy struct {
	Version  int
	From, To string
	Area     [4]float64 // x, y, w, h
	DX, DY   float64
}
