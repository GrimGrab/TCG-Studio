package gameextract

import (
	"fmt"
	"os"
	"path/filepath"

	"tcgstudio/internal/unityfs"
)

// customerIcons saves the game's customer portraits (StockItemData m_MaleCustomerIconList / m_FemaleCustomerIconList,
// shown on the tournament pairing screen) as customers\male_<model>.png / female_<model>.png, plus customers.json
// {male, female} = how many of each were saved. The Customers page previews a regular's look with them.
// CustomersDir is the templates subfolder of the customer portraits.
const CustomersDir = "customers"

func (x *extractor) customerIcons() error {
	if len(x.so.MaleCustomerIcons) == 0 && len(x.so.FemaleCustomerIcons) == 0 {
		return fmt.Errorf("no customer portraits in the item list")
	}
	dir := filepath.Join(x.dir, CustomersDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	counts := map[string]int{}
	for kind, list := range map[string][]unityfs.PPtr{"male": x.so.MaleCustomerIcons, "female": x.so.FemaleCustomerIcons} {
		for i, p := range list {
			if _, ok := x.saveSprite(x.so.File, p, filepath.Join(dir, fmt.Sprintf("%s_%d.png", kind, i))); !ok {
				break // the page only shows a contiguous run from 0
			}
			counts[kind] = i + 1
		}
	}
	return writeJSON(filepath.Join(dir, "customers.json"), counts)
}
