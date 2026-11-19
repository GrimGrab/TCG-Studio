// Links between the set editor's tabs ("Set → Card versions", "Assign cards on the Cards tab"): the editor holds one
// request, the tab it names takes it once when it shows.

export type SetTab = 'cards' | 'packs' | 'set';

export type NavRequest = {
  tab: SetTab;
  /** Set tab: the section to scroll to (its element id without the "set-" prefix). */
  section?: string;
  /** Cards tab: show only the cards no pack gives. */
  noPack?: boolean;
  /** Cards tab: start assigning cards to this pack (pack id). Packs tab: show this pack. */
  pack?: string;
};

export type Goto = (tab: SetTab, opts?: Omit<NavRequest, 'tab'>) => void;
export type TakeNav = (tab: SetTab) => NavRequest | null;
