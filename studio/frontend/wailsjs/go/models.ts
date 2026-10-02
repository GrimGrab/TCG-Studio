export namespace art {
	
	export class Options {
	    color: string;
	    title: string;
	    icon: string;
	    filePrefix: string;
	    frontImage: string;
	    titleOnImage: boolean;
	
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
	
	export class Options {
	    includeVariants: boolean;
	    imageWidth: number;
	    rarityMap: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.includeVariants = source["includeVariants"];
	        this.imageWidth = source["imageWidth"];
	        this.rarityMap = source["rarityMap"];
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

export namespace main {
	
	export class AccessoryView {
	    accessories: setfmt.Accessory[];
	    furniture: setfmt.Furniture[];
	    layouts: Record<string, string>;
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
	        this.layouts = source["layouts"];
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
	
	    static createFrom(source: any = {}) {
	        return new ModSettingsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sections = this.convertValues(source["sections"], modconfig.Section);
	        this.gameRunning = source["gameRunning"];
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
	export class ScryfallSet {
	    code: string;
	    name: string;
	    set_type: string;
	    released_at: string;
	    card_count: number;
	    digital: boolean;
	    icon_svg_uri: string;
	    parent_set_code: string;
	    imported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScryfallSet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.set_type = source["set_type"];
	        this.released_at = source["released_at"];
	        this.card_count = source["card_count"];
	        this.digital = source["digital"];
	        this.icon_svg_uri = source["icon_svg_uri"];
	        this.parent_set_code = source["parent_set_code"];
	        this.imported = source["imported"];
	    }
	}
	export class Settings {
	    gameDir: string;
	    workspace: string;
	    syncedVersion?: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.workspace = source["workspace"];
	        this.syncedVersion = source["syncedVersion"];
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

}

export namespace project {
	
	export class CardMeta {
	    scryfallId?: string;
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
	    releasedAt?: string;
	    // Go type: time
	    importedAt: any;
	    // Go type: time
	    pricesUpdated: any;
	    tier: number;
	    pricing?: Pricing;
	    cards: Record<string, CardMeta>;
	    packArt?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Meta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.scryfallCode = source["scryfallCode"];
	        this.releasedAt = source["releasedAt"];
	        this.importedAt = this.convertValues(source["importedAt"], null);
	        this.pricesUpdated = this.convertValues(source["pricesUpdated"], null);
	        this.tier = source["tier"];
	        this.pricing = this.convertValues(source["pricing"], Pricing);
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
	    set?: setfmt.Set;
	    meta?: Meta;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.folder = source["folder"];
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
	
	
	export class FurniturePoint {
	    role: string;
	    pos: number[];
	    rot: number[];
	
	    static createFrom(source: any = {}) {
	        return new FurniturePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.pos = source["pos"];
	        this.rot = source["rot"];
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
	
	    static createFrom(source: any = {}) {
	        return new PointRole(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.label = source["label"];
	        this.tip = source["tip"];
	        this.resizable = source["resizable"];
	    }
	}
	export class FurnitureType {
	    type: string;
	    title: string;
	    one: string;
	    defaultBase: string;
	    spots: string;
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
	export class PreviewPart {
	    mesh: string;
	    texture?: string;
	    glass?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PreviewPart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mesh = source["mesh"];
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

