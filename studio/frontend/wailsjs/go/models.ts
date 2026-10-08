export namespace art {
	
	export class DisplayFaces {
	    front: number[];
	    lid: number[];
	    confident: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DisplayFaces(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.front = source["front"];
	        this.lid = source["lid"];
	        this.confident = source["confident"];
	    }
	}
	export class Options {
	    color: string;
	    title: string;
	    icon: string;
	    filePrefix: string;
	    frontImage: string;
	    titleOnImage: boolean;
	    noPackText: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.color = source["color"];
	        this.title = source["title"];
	        this.icon = source["icon"];
	        this.filePrefix = source["filePrefix"];
	        this.frontImage = source["frontImage"];
	        this.titleOnImage = source["titleOnImage"];
	        this.noPackText = source["noPackText"];
	    }
	}
	export class Result {
	    packTexture: string;
	    packIcon: string;
	    boxTexture: string;
	    boxIcon: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packTexture = source["packTexture"];
	        this.packIcon = source["packIcon"];
	        this.boxTexture = source["boxTexture"];
	        this.boxIcon = source["boxIcon"];
	    }
	}

}

export namespace catalog {
	
	export class AddResult {
	    added: string[];
	    skipped: string[];
	
	    static createFrom(source: any = {}) {
	        return new AddResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.skipped = source["skipped"];
	    }
	}
	export class Deleted {
	    setups: string[];
	    setIds: string[];
	    removed: number;
	
	    static createFrom(source: any = {}) {
	        return new Deleted(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setups = source["setups"];
	        this.setIds = source["setIds"];
	        this.removed = source["removed"];
	    }
	}
	export class SetupRef {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SetupRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class Entry {
	    kind: string;
	    key: string;
	    id: string;
	    name: string;
	    sub: string;
	    detail: string;
	    origin: string;
	    author: string;
	    source: string;
	    saved: boolean;
	    from: string;
	    here: boolean;
	    usedBy: SetupRef[];
	    cover: string;
	    icon: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.key = source["key"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.sub = source["sub"];
	        this.detail = source["detail"];
	        this.origin = source["origin"];
	        this.author = source["author"];
	        this.source = source["source"];
	        this.saved = source["saved"];
	        this.from = source["from"];
	        this.here = source["here"];
	        this.usedBy = this.convertValues(source["usedBy"], SetupRef);
	        this.cover = source["cover"];
	        this.icon = source["icon"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace debuglog {
	
	export class Note {
	    level: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new Note(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.text = source["text"];
	    }
	}
	export class Report {
	    path: string;
	    found: boolean;
	    when: string;
	    size: number;
	    truncated: boolean;
	    bepinex: string;
	    loadedVersion: string;
	    errors: number;
	    notes: Note[];
	    header: string;
	    log: string;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.found = source["found"];
	        this.when = source["when"];
	        this.size = source["size"];
	        this.truncated = source["truncated"];
	        this.bepinex = source["bepinex"];
	        this.loadedVersion = source["loadedVersion"];
	        this.errors = source["errors"];
	        this.notes = this.convertValues(source["notes"], Note);
	        this.header = source["header"];
	        this.log = source["log"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace decoart {
	
	export class Poster {
	    width: number;
	    frame: number;
	    color: string;
	    depth: number;
	
	    static createFrom(source: any = {}) {
	        return new Poster(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.frame = source["frame"];
	        this.color = source["color"];
	        this.depth = source["depth"];
	    }
	}

}

export namespace epl {
	
	export class Tier {
	    name: string;
	    cards: number;
	    perPack: number;
	    perCard: number;
	    rarity: string;
	
	    static createFrom(source: any = {}) {
	        return new Tier(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.cards = source["cards"];
	        this.perPack = source["perPack"];
	        this.perCard = source["perCard"];
	        this.rarity = source["rarity"];
	    }
	}

}

export namespace figurine {
	
	export class Placement {
	    rotX: number;
	    rotY: number;
	    rotZ: number;
	    height: number;
	    anchor: number[];
	
	    static createFrom(source: any = {}) {
	        return new Placement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rotX = source["rotX"];
	        this.rotY = source["rotY"];
	        this.rotZ = source["rotZ"];
	        this.height = source["height"];
	        this.anchor = source["anchor"];
	    }
	}

}

export namespace forge {
	
	export class Status {
	    dir: string;
	    installed: boolean;
	    forgeVersion: string;
	    javaVersion: string;
	    outdated: boolean;
	    pinned: string;
	    sizeMB: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.installed = source["installed"];
	        this.forgeVersion = source["forgeVersion"];
	        this.javaVersion = source["javaVersion"];
	        this.outdated = source["outdated"];
	        this.pinned = source["pinned"];
	        this.sizeMB = source["sizeMB"];
	    }
	}

}

export namespace game {
	
	export class Status {
	    gameDir: string;
	    found: boolean;
	    bepInEx: boolean;
	    modInstalled: boolean;
	    templatesFound: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.found = source["found"];
	        this.bepInEx = source["bepInEx"];
	        this.modInstalled = source["modInstalled"];
	        this.templatesFound = source["templatesFound"];
	    }
	}

}

export namespace gamify {
	
	export class AccessoryPreview {
	    id: string;
	    name: string;
	    kind: string;
	    before: setfmt.AccessoryLicense;
	    after: setfmt.AccessoryLicense;
	    hasBig: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AccessoryPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.before = this.convertValues(source["before"], setfmt.AccessoryLicense);
	        this.after = this.convertValues(source["after"], setfmt.AccessoryLicense);
	        this.hasBig = source["hasBig"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TopCard {
	    name: string;
	    rarity: string;
	    before: number;
	    after: number;
	
	    static createFrom(source: any = {}) {
	        return new TopCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.rarity = source["rarity"];
	        this.before = source["before"];
	        this.after = source["after"];
	    }
	}
	export class Preview {
	    id: string;
	    name: string;
	    tier: number;
	    license: setfmt.License;
	    packCost: number;
	    like: string;
	    installed: boolean;
	    avgBefore: Record<string, number>;
	    avgAfter: Record<string, number>;
	    top: TopCard[];
	    locked: number;
	
	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.tier = source["tier"];
	        this.license = this.convertValues(source["license"], setfmt.License);
	        this.packCost = source["packCost"];
	        this.like = source["like"];
	        this.installed = source["installed"];
	        this.avgBefore = source["avgBefore"];
	        this.avgAfter = source["avgAfter"];
	        this.top = this.convertValues(source["top"], TopCard);
	        this.locked = source["locked"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    mode: string;
	    borderCurve: string;
	    tierStep: number;
	    packCostScale: number;
	    progression: boolean;
	    maxLevel: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.borderCurve = source["borderCurve"];
	        this.tierStep = source["tierStep"];
	        this.packCostScale = source["packCostScale"];
	        this.progression = source["progression"];
	        this.maxLevel = source["maxLevel"];
	    }
	}

}

export namespace importer {
	
	export class EPLItem {
	    key: string;
	    id: string;
	    name: string;
	    kind: string;
	    base: string;
	    exists: boolean;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new EPLItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.base = source["base"];
	        this.exists = source["exists"];
	        this.note = source["note"];
	    }
	}
	export class EPLPack {
	    name: string;
	    box: string;
	    strategy: string;
	    foilChance: number;
	
	    static createFrom(source: any = {}) {
	        return new EPLPack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.box = source["box"];
	        this.strategy = source["strategy"];
	        this.foilChance = source["foilChance"];
	    }
	}
	export class EPLSkipped {
	    name: string;
	    kind: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new EPLSkipped(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.reason = source["reason"];
	    }
	}
	export class EPLSet {
	    code: string;
	    projectId: string;
	    imported: boolean;
	    name: string;
	    cards: number;
	    tiers: epl.Tier[];
	    packs: EPLPack[];
	    renderMode: string;
	    problems: string[];
	    problemCount: number;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new EPLSet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.projectId = source["projectId"];
	        this.imported = source["imported"];
	        this.name = source["name"];
	        this.cards = source["cards"];
	        this.tiers = this.convertValues(source["tiers"], epl.Tier);
	        this.packs = this.convertValues(source["packs"], EPLPack);
	        this.renderMode = source["renderMode"];
	        this.problems = source["problems"];
	        this.problemCount = source["problemCount"];
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EPLPreview {
	    mod: string;
	    name: string;
	    modName: string;
	    sets: EPLSet[];
	    items: EPLItem[];
	    skipped: EPLSkipped[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new EPLPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mod = source["mod"];
	        this.name = source["name"];
	        this.modName = source["modName"];
	        this.sets = this.convertValues(source["sets"], EPLSet);
	        this.items = this.convertValues(source["items"], EPLItem);
	        this.skipped = this.convertValues(source["skipped"], EPLSkipped);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class Facet {
	    value: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new Facet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	    }
	}
	export class FolderSample {
	    file: string;
	    number: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new FolderSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.number = source["number"];
	        this.name = source["name"];
	    }
	}
	export class RarityCount {
	    name: string;
	    game: string;
	    cards: number;
	
	    static createFrom(source: any = {}) {
	        return new RarityCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.game = source["game"];
	        this.cards = source["cards"];
	    }
	}
	export class FolderPreview {
	    dir: string;
	    name: string;
	    projectId: string;
	    imported: boolean;
	    cards: number;
	    rarities: RarityCount[];
	    csv: boolean;
	    csvRows: number;
	    csvMatched: number;
	    unmatched: string[];
	    skipped: string[];
	    logo: boolean;
	    rotated: number;
	    offShape: number;
	    warnings: string[];
	    samples: FolderSample[];
	
	    static createFrom(source: any = {}) {
	        return new FolderPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir = source["dir"];
	        this.name = source["name"];
	        this.projectId = source["projectId"];
	        this.imported = source["imported"];
	        this.cards = source["cards"];
	        this.rarities = this.convertValues(source["rarities"], RarityCount);
	        this.csv = source["csv"];
	        this.csvRows = source["csvRows"];
	        this.csvMatched = source["csvMatched"];
	        this.unmatched = source["unmatched"];
	        this.skipped = source["skipped"];
	        this.logo = source["logo"];
	        this.rotated = source["rotated"];
	        this.offShape = source["offShape"];
	        this.warnings = source["warnings"];
	        this.samples = this.convertValues(source["samples"], FolderSample);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class IconJob {
	    id: string;
	    kind: string;
	    base: string;
	    texture: string;
	
	    static createFrom(source: any = {}) {
	        return new IconJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.base = source["base"];
	        this.texture = source["texture"];
	    }
	}
	export class Options {
	    includeVariants: boolean;
	    imageWidth: number;
	    rarityMap: Record<string, string>;
	    lang?: string;
	    imageFormat?: string;
	    useLibrary?: boolean;
	    setName?: string;
	    stripNumbers?: boolean;
	    keepRarities?: boolean;
	    originMod?: string;
	    originAuthor?: string;
	    originLink?: string;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.includeVariants = source["includeVariants"];
	        this.imageWidth = source["imageWidth"];
	        this.rarityMap = source["rarityMap"];
	        this.lang = source["lang"];
	        this.imageFormat = source["imageFormat"];
	        this.useLibrary = source["useLibrary"];
	        this.setName = source["setName"];
	        this.stripNumbers = source["stripNumbers"];
	        this.keepRarities = source["keepRarities"];
	        this.originMod = source["originMod"];
	        this.originAuthor = source["originAuthor"];
	        this.originLink = source["originLink"];
	    }
	}
	
	export class SealedGame {
	    category: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SealedGame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.name = source["name"];
	    }
	}
	export class SealedGroup {
	    id: number;
	    name: string;
	    code: string;
	    released: string;
	
	    static createFrom(source: any = {}) {
	        return new SealedGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.code = source["code"];
	        this.released = source["released"];
	    }
	}
	export class SealedProduct {
	    id: number;
	    name: string;
	    kind: string;
	    thumb: string;
	    image: string;
	
	    static createFrom(source: any = {}) {
	        return new SealedProduct(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.thumb = source["thumb"];
	        this.image = source["image"];
	    }
	}
	export class SourceInfo {
	    id: string;
	    name: string;
	    game: string;
	    languages: string[];
	    colorLabel: string;
	    colors: Facet[];
	    rarityOrder: string[];
	    sorts: Facet[];
	    variants: boolean;
	    local: boolean;
	    ownArt: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SourceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.game = source["game"];
	        this.languages = source["languages"];
	        this.colorLabel = source["colorLabel"];
	        this.colors = this.convertValues(source["colors"], Facet);
	        this.rarityOrder = source["rarityOrder"];
	        this.sorts = this.convertValues(source["sorts"], Facet);
	        this.variants = source["variants"];
	        this.local = source["local"];
	        this.ownArt = source["ownArt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace installer {
	
	export class Item {
	    id: string;
	    title: string;
	    state: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.message = source["message"];
	    }
	}
	export class State {
	    gameDir: string;
	    found: boolean;
	    running: boolean;
	    ready: boolean;
	    items: Item[];
	    version: string;
	    payload: boolean;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.found = source["found"];
	        this.running = source["running"];
	        this.ready = source["ready"];
	        this.items = this.convertValues(source["items"], Item);
	        this.version = source["version"];
	        this.payload = source["payload"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace library {
	
	export class AccessorySet {
	    id: string;
	    name: string;
	    files: number;
	    bytes: number;
	    leftoverFiles: number;
	    leftoverBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new AccessorySet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.files = source["files"];
	        this.bytes = source["bytes"];
	        this.leftoverFiles = source["leftoverFiles"];
	        this.leftoverBytes = source["leftoverBytes"];
	    }
	}
	export class AccessoryStorage {
	    storeBytes: number;
	    storeFiles: number;
	    setups: AccessorySet[];
	    moveBytes: number;
	    moveFiles: number;
	    unusedBytes: number;
	    unusedFiles: number;
	
	    static createFrom(source: any = {}) {
	        return new AccessoryStorage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.storeBytes = source["storeBytes"];
	        this.storeFiles = source["storeFiles"];
	        this.setups = this.convertValues(source["setups"], AccessorySet);
	        this.moveBytes = source["moveBytes"];
	        this.moveFiles = source["moveFiles"];
	        this.unusedBytes = source["unusedBytes"];
	        this.unusedFiles = source["unusedFiles"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Differ {
	    setup: string;
	    setupName: string;
	    set: string;
	    setName: string;
	    files: number;
	    bytes: number;
	    cards: number;
	    cardBytes: number;
	    sample: string;
	
	    static createFrom(source: any = {}) {
	        return new Differ(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setup = source["setup"];
	        this.setupName = source["setupName"];
	        this.set = source["set"];
	        this.setName = source["setName"];
	        this.files = source["files"];
	        this.bytes = source["bytes"];
	        this.cards = source["cards"];
	        this.cardBytes = source["cardBytes"];
	        this.sample = source["sample"];
	    }
	}
	export class DifferPreview {
	    file: string;
	    own: string;
	    shared: string;
	
	    static createFrom(source: any = {}) {
	        return new DifferPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.own = source["own"];
	        this.shared = source["shared"];
	    }
	}
	export class ItemInfo {
	    id: string;
	    name: string;
	    kind: string;
	    own: number;
	    shared: number;
	
	    static createFrom(source: any = {}) {
	        return new ItemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.own = source["own"];
	        this.shared = source["shared"];
	    }
	}
	export class MoveFile {
	    setup: string;
	    owner: string;
	    file: string;
	    bytes: number;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new MoveFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setup = source["setup"];
	        this.owner = source["owner"];
	        this.file = source["file"];
	        this.bytes = source["bytes"];
	        this.action = source["action"];
	    }
	}
	export class Progress {
	    message: string;
	    done: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new Progress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.done = source["done"];
	        this.total = source["total"];
	    }
	}
	export class UnusedEntry {
	    kind: string;
	    id: string;
	    name: string;
	    size: number;
	    files: number;
	    inCatalog: boolean;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new UnusedEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.files = source["files"];
	        this.inCatalog = source["inCatalog"];
	        this.note = source["note"];
	    }
	}
	export class SetInfo {
	    id: string;
	    name: string;
	    own: number;
	    library: number;
	    shared: number;
	    differs: number;
	    png: number;
	    pngFiles: number;
	    shrinkTo: number;
	    movable: number;
	    shareable: boolean;
	    artFolder: string;
	
	    static createFrom(source: any = {}) {
	        return new SetInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.own = source["own"];
	        this.library = source["library"];
	        this.shared = source["shared"];
	        this.differs = source["differs"];
	        this.png = source["png"];
	        this.pngFiles = source["pngFiles"];
	        this.shrinkTo = source["shrinkTo"];
	        this.movable = source["movable"];
	        this.shareable = source["shareable"];
	        this.artFolder = source["artFolder"];
	    }
	}
	export class SetupInfo {
	    id: string;
	    name: string;
	    active: boolean;
	    size: number;
	    info: number;
	    cardArt: number;
	    setFiles: number;
	    leftovers: number;
	    itemFiles: number;
	    saves: number;
	    game: number;
	    other: number;
	    sets: SetInfo[];
	    items: ItemInfo[];
	
	    static createFrom(source: any = {}) {
	        return new SetupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.active = source["active"];
	        this.size = source["size"];
	        this.info = source["info"];
	        this.cardArt = source["cardArt"];
	        this.setFiles = source["setFiles"];
	        this.leftovers = source["leftovers"];
	        this.itemFiles = source["itemFiles"];
	        this.saves = source["saves"];
	        this.game = source["game"];
	        this.other = source["other"];
	        this.sets = this.convertValues(source["sets"], SetInfo);
	        this.items = this.convertValues(source["items"], ItemInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Report {
	    workspace: number;
	    library: number;
	    assets: number;
	    catalog: number;
	    gameSets: number;
	    gameLibrary: number;
	    gameFound: boolean;
	    modHasLibrary: boolean;
	    setups: SetupInfo[];
	    moveSaves: number;
	    moveFiles: number;
	    moveBytes: number;
	    moveList: MoveFile[];
	    differ: Differ[];
	    unused: UnusedEntry[];
	    accessories: AccessoryStorage;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = source["workspace"];
	        this.library = source["library"];
	        this.assets = source["assets"];
	        this.catalog = source["catalog"];
	        this.gameSets = source["gameSets"];
	        this.gameLibrary = source["gameLibrary"];
	        this.gameFound = source["gameFound"];
	        this.modHasLibrary = source["modHasLibrary"];
	        this.setups = this.convertValues(source["setups"], SetupInfo);
	        this.moveSaves = source["moveSaves"];
	        this.moveFiles = source["moveFiles"];
	        this.moveBytes = source["moveBytes"];
	        this.moveList = this.convertValues(source["moveList"], MoveFile);
	        this.differ = this.convertValues(source["differ"], Differ);
	        this.unused = this.convertValues(source["unused"], UnusedEntry);
	        this.accessories = this.convertValues(source["accessories"], AccessoryStorage);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ShrinkPreview {
	    card: string;
	    png: string;
	    jpeg: string;
	    pngSize: number;
	    jpegSize: number;
	
	    static createFrom(source: any = {}) {
	        return new ShrinkPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.card = source["card"];
	        this.png = source["png"];
	        this.jpeg = source["jpeg"];
	        this.pngSize = source["pngSize"];
	        this.jpegSize = source["jpegSize"];
	    }
	}

}

export namespace main {
	
	export class AccessoryView {
	    accessories: setfmt.Accessory[];
	    furniture: setfmt.Furniture[];
	    decorations: setfmt.Decoration[];
	    layouts: Record<string, string>;
	    origins: Record<string, origin.Origin>;
	    installed: boolean;
	    warnings: string[];
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new AccessoryView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accessories = this.convertValues(source["accessories"], setfmt.Accessory);
	        this.furniture = this.convertValues(source["furniture"], setfmt.Furniture);
	        this.decorations = this.convertValues(source["decorations"], setfmt.Decoration);
	        this.layouts = source["layouts"];
	        this.origins = this.convertValues(source["origins"], origin.Origin, true);
	        this.installed = source["installed"];
	        this.warnings = source["warnings"];
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BoxArt {
	    boxTexture: string;
	    boxIcon: string;
	    layout: string;
	
	    static createFrom(source: any = {}) {
	        return new BoxArt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.boxTexture = source["boxTexture"];
	        this.boxIcon = source["boxIcon"];
	        this.layout = source["layout"];
	    }
	}
	export class DecorationBake {
	    mesh: string;
	    texture: string;
	    triangles: number;
	    size: number[];
	
	    static createFrom(source: any = {}) {
	        return new DecorationBake(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mesh = source["mesh"];
	        this.texture = source["texture"];
	        this.triangles = source["triangles"];
	        this.size = source["size"];
	    }
	}
	export class EPLResult {
	    sets: string[];
	    items: number;
	    icons: importer.IconJob[];
	    problems: string[];
	
	    static createFrom(source: any = {}) {
	        return new EPLResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sets = source["sets"];
	        this.items = source["items"];
	        this.icons = this.convertValues(source["icons"], importer.IconJob);
	        this.problems = source["problems"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EPLSelection {
	    sets: string[];
	    rarity: Record<string, any>;
	    items: string[];
	    options: importer.Options;
	
	    static createFrom(source: any = {}) {
	        return new EPLSelection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sets = source["sets"];
	        this.rarity = source["rarity"];
	        this.items = source["items"];
	        this.options = this.convertValues(source["options"], importer.Options);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FigurineBake {
	    mesh: string;
	    texture: string;
	    triangles: number;
	    size: number[];
	
	    static createFrom(source: any = {}) {
	        return new FigurineBake(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mesh = source["mesh"];
	        this.texture = source["texture"];
	        this.triangles = source["triangles"];
	        this.size = source["size"];
	    }
	}
	export class FigurineSource {
	    model: string;
	    texture: string;
	    name: string;
	    triangles: number;
	    vertices: number;
	    size: number[];
	    warnings: string[];
	    file: string;
	
	    static createFrom(source: any = {}) {
	        return new FigurineSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.texture = source["texture"];
	        this.name = source["name"];
	        this.triangles = source["triangles"];
	        this.vertices = source["vertices"];
	        this.size = source["size"];
	        this.warnings = source["warnings"];
	        this.file = source["file"];
	    }
	}
	export class FurnitureBake {
	    mesh: string;
	    texture: string;
	    triangles: number;
	    size: number[];
	
	    static createFrom(source: any = {}) {
	        return new FurnitureBake(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mesh = source["mesh"];
	        this.texture = source["texture"];
	        this.triangles = source["triangles"];
	        this.size = source["size"];
	    }
	}
	export class FurniturePaintInfo {
	    template: string;
	    model: uvmap.Model;
	    vanilla: string;
	    parts: setfmt.PaintPart[];
	
	    static createFrom(source: any = {}) {
	        return new FurniturePaintInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.template = source["template"];
	        this.model = this.convertValues(source["model"], uvmap.Model);
	        this.vanilla = source["vanilla"];
	        this.parts = this.convertValues(source["parts"], setfmt.PaintPart);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ImportableSet {
	    code: string;
	    name: string;
	    group: string;
	    releasedAt: string;
	    icon: string;
	    iconMono: boolean;
	    cards: number;
	    main: boolean;
	    imported: boolean;
	    projectId: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportableSet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.releasedAt = source["releasedAt"];
	        this.icon = source["icon"];
	        this.iconMono = source["iconMono"];
	        this.cards = source["cards"];
	        this.main = source["main"];
	        this.imported = source["imported"];
	        this.projectId = source["projectId"];
	    }
	}
	export class LibraryArtInfo {
	    format: string;
	    width: number;
	    cards: number;
	
	    static createFrom(source: any = {}) {
	        return new LibraryArtInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.width = source["width"];
	        this.cards = source["cards"];
	    }
	}
	export class ModCheck {
	    logFound: boolean;
	    when: string;
	    loaded: boolean;
	    summary: string;
	    errors: string[];
	
	    static createFrom(source: any = {}) {
	        return new ModCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.logFound = source["logFound"];
	        this.when = source["when"];
	        this.loaded = source["loaded"];
	        this.summary = source["summary"];
	        this.errors = source["errors"];
	    }
	}
	export class ModSettingsView {
	    sections: modconfig.Section[];
	    gameRunning: boolean;
	    showWhen: Record<string, modconfig.ShowWhen>;
	    setup: string;
	
	    static createFrom(source: any = {}) {
	        return new ModSettingsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sections = this.convertValues(source["sections"], modconfig.Section);
	        this.gameRunning = source["gameRunning"];
	        this.showWhen = this.convertValues(source["showWhen"], modconfig.ShowWhen, true);
	        this.setup = source["setup"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelTextures {
	    color: string;
	    normal: string;
	    normalDirectX: boolean;
	    ao: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelTextures(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.color = source["color"];
	        this.normal = source["normal"];
	        this.normalDirectX = source["normalDirectX"];
	        this.ao = source["ao"];
	    }
	}
	export class PackArtFiles {
	    texture: string;
	    icon: string;
	
	    static createFrom(source: any = {}) {
	        return new PackArtFiles(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.texture = source["texture"];
	        this.icon = source["icon"];
	    }
	}
	export class PickedProducts {
	    pack?: importer.SealedProduct;
	    box?: importer.SealedProduct;
	
	    static createFrom(source: any = {}) {
	        return new PickedProducts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pack = this.convertValues(source["pack"], importer.SealedProduct);
	        this.box = this.convertValues(source["box"], importer.SealedProduct);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProductPhotoSets {
	    games: importer.SealedGame[];
	    category: number;
	    groups: importer.SealedGroup[];
	    match: number;
	
	    static createFrom(source: any = {}) {
	        return new ProductPhotoSets(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.games = this.convertValues(source["games"], importer.SealedGame);
	        this.category = source["category"];
	        this.groups = this.convertValues(source["groups"], importer.SealedGroup);
	        this.match = source["match"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveResult {
	    issues: setfmt.Issue[];
	    synced: boolean;
	    installState: string;
	    gameRunning: boolean;
	    syncError: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.issues = this.convertValues(source["issues"], setfmt.Issue);
	        this.synced = source["synced"];
	        this.installState = source["installState"];
	        this.gameRunning = source["gameRunning"];
	        this.syncError = source["syncError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    gameDir: string;
	    workspace: string;
	    syncedVersion?: string;
	    imageFormat?: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.workspace = source["workspace"];
	        this.syncedVersion = source["syncedVersion"];
	        this.imageFormat = source["imageFormat"];
	    }
	}
	export class SetupFileInfo {
	    file: string;
	    manifest?: setups.Manifest;
	
	    static createFrom(source: any = {}) {
	        return new SetupFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.manifest = this.convertValues(source["manifest"], setups.Manifest);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SetupsView {
	    setups: setups.Summary[];
	    active: string;
	    gameRunning: boolean;
	    gameFound: boolean;
	    steamCloud: boolean;
	    error: string;
	    message: string;
	    pending: boolean;
	    workspace: string;
	
	    static createFrom(source: any = {}) {
	        return new SetupsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setups = this.convertValues(source["setups"], setups.Summary);
	        this.active = source["active"];
	        this.gameRunning = source["gameRunning"];
	        this.gameFound = source["gameFound"];
	        this.steamCloud = source["steamCloud"];
	        this.error = source["error"];
	        this.message = source["message"];
	        this.pending = source["pending"];
	        this.workspace = source["workspace"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SmartArtCard {
	    name: string;
	    image: string;
	    window: number[];
	
	    static createFrom(source: any = {}) {
	        return new SmartArtCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.image = source["image"];
	        this.window = source["window"];
	    }
	}
	export class SmartArtSources {
	    packPhoto: string;
	    packName: string;
	    boxPhoto: string;
	    boxName: string;
	    box?: art.DisplayFaces;
	    cards: SmartArtCard[];
	    icon: string;
	    notes: string[];
	
	    static createFrom(source: any = {}) {
	        return new SmartArtSources(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packPhoto = source["packPhoto"];
	        this.packName = source["packName"];
	        this.boxPhoto = source["boxPhoto"];
	        this.boxName = source["boxName"];
	        this.box = this.convertValues(source["box"], art.DisplayFaces);
	        this.cards = this.convertValues(source["cards"], SmartArtCard);
	        this.icon = source["icon"];
	        this.notes = source["notes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SmartChoice {
	    manual: boolean;
	    pack?: importer.SealedProduct;
	    box?: importer.SealedProduct;
	
	    static createFrom(source: any = {}) {
	        return new SmartChoice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.manual = source["manual"];
	        this.pack = this.convertValues(source["pack"], importer.SealedProduct);
	        this.box = this.convertValues(source["box"], importer.SealedProduct);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StorageResult {
	    files: number;
	    freed: number;
	    sets: number;
	    reinstalled: number;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new StorageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = source["files"];
	        this.freed = source["freed"];
	        this.sets = source["sets"];
	        this.reinstalled = source["reinstalled"];
	        this.note = source["note"];
	    }
	}
	export class StorageTaskInfo {
	    running: boolean;
	    label: string;
	    progress: library.Progress;
	
	    static createFrom(source: any = {}) {
	        return new StorageTaskInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.label = source["label"];
	        this.progress = this.convertValues(source["progress"], library.Progress);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SwitchResult {
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new SwitchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.warnings = source["warnings"];
	    }
	}
	export class TemplatesStatus {
	    ready: boolean;
	    source: string;
	    busy: boolean;
	    message: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new TemplatesStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ready = source["ready"];
	        this.source = source["source"];
	        this.busy = source["busy"];
	        this.message = source["message"];
	        this.error = source["error"];
	    }
	}
	export class VersionInfo {
	    version: string;
	    canUpdate: boolean;
	    updatedTo: string;
	    releaseUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.canUpdate = source["canUpdate"];
	        this.updatedTo = source["updatedTo"];
	        this.releaseUrl = source["releaseUrl"];
	    }
	}

}

export namespace modconfig {
	
	export class Entry {
	    section: string;
	    key: string;
	    value: string;
	    type: string;
	    default: string;
	    description: string;
	    min?: number;
	    max?: number;
	    options?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.section = source["section"];
	        this.key = source["key"];
	        this.value = source["value"];
	        this.type = source["type"];
	        this.default = source["default"];
	        this.description = source["description"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.options = source["options"];
	    }
	}
	export class Section {
	    name: string;
	    entries: Entry[];
	
	    static createFrom(source: any = {}) {
	        return new Section(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.entries = this.convertValues(source["entries"], Entry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ShowWhen {
	    setting: string;
	    is: string[];
	
	    static createFrom(source: any = {}) {
	        return new ShowWhen(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setting = source["setting"];
	        this.is = source["is"];
	    }
	}

}

export namespace origin {
	
	export class Origin {
	    kind: string;
	    mod: string;
	    author?: string;
	    link?: string;
	    package?: string;
	    path?: string;
	    bundle?: string;
	    item?: string;
	    // Go type: time
	    imported: any;
	
	    static createFrom(source: any = {}) {
	        return new Origin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.mod = source["mod"];
	        this.author = source["author"];
	        this.link = source["link"];
	        this.package = source["package"];
	        this.path = source["path"];
	        this.bundle = source["bundle"];
	        this.item = source["item"];
	        this.imported = this.convertValues(source["imported"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace project {
	
	export class CardMeta {
	    scryfallId?: string;
	    sourceId?: string;
	    name?: string;
	    layout?: string;
	    colors?: string[];
	    typeLine?: string;
	    manaCost?: string;
	    cmc?: number;
	    power?: string;
	    toughness?: string;
	    srcRarity?: string;
	    variant?: string[];
	    usd?: number;
	    usdFoil?: number;
	    eur?: number;
	    locked?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CardMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scryfallId = source["scryfallId"];
	        this.sourceId = source["sourceId"];
	        this.name = source["name"];
	        this.layout = source["layout"];
	        this.colors = source["colors"];
	        this.typeLine = source["typeLine"];
	        this.manaCost = source["manaCost"];
	        this.cmc = source["cmc"];
	        this.power = source["power"];
	        this.toughness = source["toughness"];
	        this.srcRarity = source["srcRarity"];
	        this.variant = source["variant"];
	        this.usd = source["usd"];
	        this.usdFoil = source["usdFoil"];
	        this.eur = source["eur"];
	        this.locked = source["locked"];
	    }
	}
	export class Pricing {
	    mode: string;
	    borderCurve: string;
	    tierStep: number;
	    position: number;
	
	    static createFrom(source: any = {}) {
	        return new Pricing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.borderCurve = source["borderCurve"];
	        this.tierStep = source["tierStep"];
	        this.position = source["position"];
	    }
	}
	export class Meta {
	    source: string;
	    scryfallCode?: string;
	    setCode?: string;
	    sourceDir?: string;
	    rarityOrder?: string[];
	    origin?: origin.Origin;
	    lang?: string;
	    releasedAt?: string;
	    // Go type: time
	    importedAt: any;
	    // Go type: time
	    pricesUpdated: any;
	    tier: number;
	    pricing?: Pricing;
	    srcPriceDefaults?: setfmt.PriceDefaults;
	    cards: Record<string, CardMeta>;
	    packArt?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Meta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.scryfallCode = source["scryfallCode"];
	        this.setCode = source["setCode"];
	        this.sourceDir = source["sourceDir"];
	        this.rarityOrder = source["rarityOrder"];
	        this.origin = this.convertValues(source["origin"], origin.Origin);
	        this.lang = source["lang"];
	        this.releasedAt = source["releasedAt"];
	        this.importedAt = this.convertValues(source["importedAt"], null);
	        this.pricesUpdated = this.convertValues(source["pricesUpdated"], null);
	        this.tier = source["tier"];
	        this.pricing = this.convertValues(source["pricing"], Pricing);
	        this.srcPriceDefaults = this.convertValues(source["srcPriceDefaults"], setfmt.PriceDefaults);
	        this.cards = this.convertValues(source["cards"], CardMeta, true);
	        this.packArt = source["packArt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Project {
	    id: string;
	    folder: string;
	    libFolder: string;
	    set?: setfmt.Set;
	    meta?: Meta;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.folder = source["folder"];
	        this.libFolder = source["libFolder"];
	        this.set = this.convertValues(source["set"], setfmt.Set);
	        this.meta = this.convertValues(source["meta"], Meta);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Summary {
	    id: string;
	    name: string;
	    cards: number;
	    packs: number;
	    source: string;
	    code: string;
	    releasedAt: string;
	    tier: number;
	    // Go type: time
	    modified: any;
	    installed: boolean;
	    installState: string;
	    cover: string;
	    origin?: string;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.cards = source["cards"];
	        this.packs = source["packs"];
	        this.source = source["source"];
	        this.code = source["code"];
	        this.releasedAt = source["releasedAt"];
	        this.tier = source["tier"];
	        this.modified = this.convertValues(source["modified"], null);
	        this.installed = source["installed"];
	        this.installState = source["installState"];
	        this.cover = source["cover"];
	        this.origin = source["origin"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace setfmt {
	
	export class AccessoryLicense {
	    level: number;
	    price: number;
	    bigLevel?: number;
	    bigPrice?: number;
	
	    static createFrom(source: any = {}) {
	        return new AccessoryLicense(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.price = source["price"];
	        this.bigLevel = source["bigLevel"];
	        this.bigPrice = source["bigPrice"];
	    }
	}
	export class Accessory {
	    id: string;
	    kind: string;
	    name: string;
	    base?: string;
	    texture?: string;
	    icon?: string;
	    mesh?: string;
	    cost?: number;
	    marketMin?: number;
	    marketMax?: number;
	    license: AccessoryLicense;
	
	    static createFrom(source: any = {}) {
	        return new Accessory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.base = source["base"];
	        this.texture = source["texture"];
	        this.icon = source["icon"];
	        this.mesh = source["mesh"];
	        this.cost = source["cost"];
	        this.marketMin = source["marketMin"];
	        this.marketMax = source["marketMax"];
	        this.license = this.convertValues(source["license"], AccessoryLicense);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AccessoryKind {
	    kind: string;
	    title: string;
	    one: string;
	    toggle: string;
	    bases: string[];
	    palette: boolean;
	    model: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AccessoryKind(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.one = source["one"];
	        this.toggle = source["toggle"];
	        this.bases = source["bases"];
	        this.palette = source["palette"];
	        this.model = source["model"];
	    }
	}
	
	export class CardMtg {
	    name: string;
	    typeLine?: string;
	    manaCost?: string;
	    colors?: string[];
	    rarity?: string;
	    cmc?: number;
	    power?: string;
	    toughness?: string;
	
	    static createFrom(source: any = {}) {
	        return new CardMtg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.typeLine = source["typeLine"];
	        this.manaCost = source["manaCost"];
	        this.colors = source["colors"];
	        this.rarity = source["rarity"];
	        this.cmc = source["cmc"];
	        this.power = source["power"];
	        this.toughness = source["toughness"];
	    }
	}
	export class Play {
	    laneAttack: number[];
	    element: string;
	    evolvesFrom?: string;
	    effect?: number[];
	
	    static createFrom(source: any = {}) {
	        return new Play(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.laneAttack = source["laneAttack"];
	        this.element = source["element"];
	        this.evolvesFrom = source["evolvesFrom"];
	        this.effect = source["effect"];
	    }
	}
	export class CardPrice {
	    base: number;
	    foilMultiplier?: number;
	    borderMultipliers?: number[];
	    overrides?: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new CardPrice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base = source["base"];
	        this.foilMultiplier = source["foilMultiplier"];
	        this.borderMultipliers = source["borderMultipliers"];
	        this.overrides = source["overrides"];
	    }
	}
	export class Card {
	    id: string;
	    name: string;
	    description: string;
	    artist: string;
	    rarity: string;
	    number?: string;
	    image: string;
	    price: CardPrice;
	    play: Play;
	    mtg?: CardMtg;
	
	    static createFrom(source: any = {}) {
	        return new Card(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.artist = source["artist"];
	        this.rarity = source["rarity"];
	        this.number = source["number"];
	        this.image = source["image"];
	        this.price = this.convertValues(source["price"], CardPrice);
	        this.play = this.convertValues(source["play"], Play);
	        this.mtg = this.convertValues(source["mtg"], CardMtg);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class Decoration {
	    id: string;
	    kind: string;
	    name: string;
	    price: number;
	    icon?: string;
	    texture?: string;
	    normalMap?: string;
	    roughnessMap?: string;
	    color?: string;
	    smoothness?: number;
	    mesh?: string;
	    mount?: string;
	    tab?: string;
	
	    static createFrom(source: any = {}) {
	        return new Decoration(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.icon = source["icon"];
	        this.texture = source["texture"];
	        this.normalMap = source["normalMap"];
	        this.roughnessMap = source["roughnessMap"];
	        this.color = source["color"];
	        this.smoothness = source["smoothness"];
	        this.mesh = source["mesh"];
	        this.mount = source["mount"];
	        this.tab = source["tab"];
	    }
	}
	export class DecorationKind {
	    kind: string;
	    title: string;
	    one: string;
	    toggle: string;
	    surface: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DecorationKind(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.one = source["one"];
	        this.toggle = source["toggle"];
	        this.surface = source["surface"];
	    }
	}
	export class FurniturePoint {
	    role: string;
	    pos: number[];
	    rot: number[];
	    scale?: number;
	
	    static createFrom(source: any = {}) {
	        return new FurniturePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.pos = source["pos"];
	        this.rot = source["rot"];
	        this.scale = source["scale"];
	    }
	}
	export class FurnitureArea {
	    pos: number[];
	    size: number[];
	
	    static createFrom(source: any = {}) {
	        return new FurnitureArea(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pos = source["pos"];
	        this.size = source["size"];
	    }
	}
	export class FurnitureSpot {
	    kind: string;
	    pos: number[];
	    rot: number[];
	    size?: number[];
	    grid?: number[];
	    customer?: number[];
	    priceTag?: number[];
	    boxes?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FurnitureSpot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.pos = source["pos"];
	        this.rot = source["rot"];
	        this.size = source["size"];
	        this.grid = source["grid"];
	        this.customer = source["customer"];
	        this.priceTag = source["priceTag"];
	        this.boxes = source["boxes"];
	    }
	}
	export class PaintPart {
	    renderer: string;
	    mesh: string;
	
	    static createFrom(source: any = {}) {
	        return new PaintPart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.renderer = source["renderer"];
	        this.mesh = source["mesh"];
	    }
	}
	export class FurniturePaint {
	    texture: string;
	    parts: PaintPart[];
	
	    static createFrom(source: any = {}) {
	        return new FurniturePaint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.texture = source["texture"];
	        this.parts = this.convertValues(source["parts"], PaintPart);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Furniture {
	    id: string;
	    type: string;
	    name: string;
	    description?: string;
	    base?: string;
	    price?: number;
	    level?: number;
	    decoBonus?: number;
	    icon?: string;
	    texture?: string;
	    tint?: string;
	    mesh?: string;
	    paint?: FurniturePaint;
	    spots?: FurnitureSpot[];
	    area?: FurnitureArea;
	    points?: FurniturePoint[];
	
	    static createFrom(source: any = {}) {
	        return new Furniture(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.base = source["base"];
	        this.price = source["price"];
	        this.level = source["level"];
	        this.decoBonus = source["decoBonus"];
	        this.icon = source["icon"];
	        this.texture = source["texture"];
	        this.tint = source["tint"];
	        this.mesh = source["mesh"];
	        this.paint = this.convertValues(source["paint"], FurniturePaint);
	        this.spots = this.convertValues(source["spots"], FurnitureSpot);
	        this.area = this.convertValues(source["area"], FurnitureArea);
	        this.points = this.convertValues(source["points"], FurniturePoint);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class PointRole {
	    role: string;
	    label: string;
	    tip: string;
	    resizable: boolean;
	    kind: string;
	    parent?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PointRole(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.label = source["label"];
	        this.tip = source["tip"];
	        this.resizable = source["resizable"];
	        this.kind = source["kind"];
	        this.parent = source["parent"];
	    }
	}
	export class FurnitureType {
	    type: string;
	    title: string;
	    one: string;
	    defaultBase: string;
	    spots: string[];
	    points: PointRole[];
	
	    static createFrom(source: any = {}) {
	        return new FurnitureType(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.title = source["title"];
	        this.one = source["one"];
	        this.defaultBase = source["defaultBase"];
	        this.spots = source["spots"];
	        this.points = this.convertValues(source["points"], PointRole);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Issue {
	    level: string;
	    where: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.where = source["where"];
	        this.message = source["message"];
	    }
	}
	export class License {
	    packLevel: number;
	    packPrice: number;
	    packBigLevel?: number;
	    packBigPrice?: number;
	    boxLevel: number;
	    boxPrice: number;
	    boxBigLevel?: number;
	    boxBigPrice?: number;
	
	    static createFrom(source: any = {}) {
	        return new License(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packLevel = source["packLevel"];
	        this.packPrice = source["packPrice"];
	        this.packBigLevel = source["packBigLevel"];
	        this.packBigPrice = source["packBigPrice"];
	        this.boxLevel = source["boxLevel"];
	        this.boxPrice = source["boxPrice"];
	        this.boxBigLevel = source["boxBigLevel"];
	        this.boxBigPrice = source["boxBigPrice"];
	    }
	}
	export class Slot {
	    count: number;
	    weights: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new Slot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.weights = source["weights"];
	    }
	}
	export class Pack {
	    id: string;
	    name: string;
	    boxName?: string;
	    cardsPerPack: number;
	    starter: boolean;
	    hasBox: boolean;
	    packTexture?: string;
	    packIcon?: string;
	    boxTexture?: string;
	    boxIcon?: string;
	    packCost: number;
	    boxCost?: number;
	    marketMin: number;
	    marketMax: number;
	    license: License;
	    slots: Slot[];
	    foilChance: number;
	    borderOdds: Record<string, number>;
	    allowDuplicates: boolean;
	    cards: string[];
	
	    static createFrom(source: any = {}) {
	        return new Pack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.boxName = source["boxName"];
	        this.cardsPerPack = source["cardsPerPack"];
	        this.starter = source["starter"];
	        this.hasBox = source["hasBox"];
	        this.packTexture = source["packTexture"];
	        this.packIcon = source["packIcon"];
	        this.boxTexture = source["boxTexture"];
	        this.boxIcon = source["boxIcon"];
	        this.packCost = source["packCost"];
	        this.boxCost = source["boxCost"];
	        this.marketMin = source["marketMin"];
	        this.marketMax = source["marketMax"];
	        this.license = this.convertValues(source["license"], License);
	        this.slots = this.convertValues(source["slots"], Slot);
	        this.foilChance = source["foilChance"];
	        this.borderOdds = source["borderOdds"];
	        this.allowDuplicates = source["allowDuplicates"];
	        this.cards = source["cards"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class PriceDefaults {
	    borderMultipliers: number[];
	    foilMultiplier: number;
	    minimum: number;
	
	    static createFrom(source: any = {}) {
	        return new PriceDefaults(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.borderMultipliers = source["borderMultipliers"];
	        this.foilMultiplier = source["foilMultiplier"];
	        this.minimum = source["minimum"];
	    }
	}
	export class Rarity {
	    id: string;
	    name: string;
	    color?: string;
	
	    static createFrom(source: any = {}) {
	        return new Rarity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	    }
	}
	export class SetMtg {
	    setCode: string;
	
	    static createFrom(source: any = {}) {
	        return new SetMtg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setCode = source["setCode"];
	    }
	}
	export class Set {
	    schemaVersion: number;
	    id: string;
	    name: string;
	    renderMode: string;
	    frameTemplate: string;
	    cardBack?: string;
	    mtg?: SetMtg;
	    rarities?: Rarity[];
	    priceDefaults: PriceDefaults;
	    packs: Pack[];
	    cards: Card[];
	
	    static createFrom(source: any = {}) {
	        return new Set(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.renderMode = source["renderMode"];
	        this.frameTemplate = source["frameTemplate"];
	        this.cardBack = source["cardBack"];
	        this.mtg = this.convertValues(source["mtg"], SetMtg);
	        this.rarities = this.convertValues(source["rarities"], Rarity);
	        this.priceDefaults = this.convertValues(source["priceDefaults"], PriceDefaults);
	        this.packs = this.convertValues(source["packs"], Pack);
	        this.cards = this.convertValues(source["cards"], Card);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace setups {
	
	export class Manifest {
	    format: string;
	    formatVersion: number;
	    name: string;
	    description?: string;
	    studioVersion?: string;
	    // Go type: time
	    exported: any;
	    sets: number;
	    accessories: number;
	    furniture: number;
	    decorations?: number;
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.formatVersion = source["formatVersion"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.studioVersion = source["studioVersion"];
	        this.exported = this.convertValues(source["exported"], null);
	        this.sets = source["sets"];
	        this.accessories = source["accessories"];
	        this.furniture = source["furniture"];
	        this.decorations = source["decorations"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Summary {
	    id: string;
	    name: string;
	    description: string;
	    // Go type: time
	    created: any;
	    folder: string;
	    active: boolean;
	    sets: number;
	    accessories: number;
	    furniture: number;
	    decorations: number;
	    hasSaves: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.created = this.convertValues(source["created"], null);
	        this.folder = source["folder"];
	        this.active = source["active"];
	        this.sets = source["sets"];
	        this.accessories = source["accessories"];
	        this.furniture = source["furniture"];
	        this.decorations = source["decorations"];
	        this.hasSaves = source["hasSaves"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace updater {
	
	export class Release {
	    version: string;
	    notes: string;
	    url: string;
	    size: number;
	    sha256: string;
	    page: string;
	
	    static createFrom(source: any = {}) {
	        return new Release(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.notes = source["notes"];
	        this.url = source["url"];
	        this.size = source["size"];
	        this.sha256 = source["sha256"];
	        this.page = source["page"];
	    }
	}

}

export namespace uvmap {
	
	export class Base {
	    id: string;
	    label: string;
	    texture: string;
	    targets?: Record<string, Array<Target>>;
	
	    static createFrom(source: any = {}) {
	        return new Base(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.texture = source["texture"];
	        this.targets = this.convertValues(source["targets"], Array<Target>, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Target {
	    rect: number[];
	    src: number[];
	    flipX?: boolean;
	    flipY?: boolean;
	    transpose?: boolean;
	    bleed?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rect = source["rect"];
	        this.src = source["src"];
	        this.flipX = source["flipX"];
	        this.flipY = source["flipY"];
	        this.transpose = source["transpose"];
	        this.bleed = source["bleed"];
	    }
	}
	export class Face {
	    id: string;
	    label: string;
	    net: number[];
	    hidden?: boolean;
	    targets: Target[];
	
	    static createFrom(source: any = {}) {
	        return new Face(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.net = source["net"];
	        this.hidden = source["hidden"];
	        this.targets = this.convertValues(source["targets"], Target);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class View {
	    label: string;
	    net: number[];
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.net = source["net"];
	    }
	}
	export class PreviewPart {
	    mesh: string;
	    url?: string;
	    texture?: string;
	    glass?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PreviewPart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mesh = source["mesh"];
	        this.url = source["url"];
	        this.texture = source["texture"];
	        this.glass = source["glass"];
	    }
	}
	export class Model {
	    kind: string;
	    mesh: string;
	    textureSize: number;
	    size: number[];
	    faces: Face[];
	    palette?: boolean;
	    preview: PreviewPart[];
	    icon: string;
	    bases?: Base[];
	    projected?: boolean;
	    views?: View[];
	    density?: number;
	    turn?: number;
	
	    static createFrom(source: any = {}) {
	        return new Model(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.mesh = source["mesh"];
	        this.textureSize = source["textureSize"];
	        this.size = source["size"];
	        this.faces = this.convertValues(source["faces"], Face);
	        this.palette = source["palette"];
	        this.preview = this.convertValues(source["preview"], PreviewPart);
	        this.icon = source["icon"];
	        this.bases = this.convertValues(source["bases"], Base);
	        this.projected = source["projected"];
	        this.views = this.convertValues(source["views"], View);
	        this.density = source["density"];
	        this.turn = source["turn"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

