export namespace models {
	
	export class BotState {
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BotState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	    }
	}
	export class WatchlistFeed {
	    provider: string;
	    url: string;
	    title?: string;
	    external_id?: string;
	    last_refreshed_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistFeed(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.external_id = source["external_id"];
	        this.last_refreshed_at = source["last_refreshed_at"];
	    }
	}
	export class CockpitStrategySummary {
	    id: string;
	    name: string;
	    rule_count: number;
	    enabled_rule_count: number;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new CockpitStrategySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.rule_count = source["rule_count"];
	        this.enabled_rule_count = source["enabled_rule_count"];
	        this.source = source["source"];
	    }
	}
	export class CockpitStrategySlot {
	    id: string;
	    label: string;
	    strategy_id?: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CockpitStrategySlot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.strategy_id = source["strategy_id"];
	        this.enabled = source["enabled"];
	    }
	}
	export class CockpitWorkspace {
	    id: string;
	    name: string;
	    kind: string;
	    enabled: boolean;
	    strategy_slots: CockpitStrategySlot[];
	
	    static createFrom(source: any = {}) {
	        return new CockpitWorkspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.enabled = source["enabled"];
	        this.strategy_slots = this.convertValues(source["strategy_slots"], CockpitStrategySlot);
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
	export class CockpitStateResponse {
	    global_enabled: boolean;
	    active_workspace_id?: string;
	    workspaces: CockpitWorkspace[];
	    strategy_library: CockpitStrategySummary[];
	    feed?: WatchlistFeed;
	    updated_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new CockpitStateResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.global_enabled = source["global_enabled"];
	        this.active_workspace_id = source["active_workspace_id"];
	        this.workspaces = this.convertValues(source["workspaces"], CockpitWorkspace);
	        this.strategy_library = this.convertValues(source["strategy_library"], CockpitStrategySummary);
	        this.feed = this.convertValues(source["feed"], WatchlistFeed);
	        this.updated_at = source["updated_at"];
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
	
	
	
	
	export class WatchlistItem {
	    symbol: string;
	    exchange: string;
	    name: string;
	    enabled: boolean;
	    instrument_id?: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.exchange = source["exchange"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.instrument_id = source["instrument_id"];
	    }
	}
	export class WatchlistGroup {
	    id: string;
	    name: string;
	    source: string;
	    items: WatchlistItem[];
	
	    static createFrom(source: any = {}) {
	        return new WatchlistGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.source = source["source"];
	        this.items = this.convertValues(source["items"], WatchlistItem);
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
	
	export class WatchlistResponse {
	    symbols: string[];
	    groups: WatchlistGroup[];
	    feed?: WatchlistFeed;
	    updated_at?: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbols = source["symbols"];
	        this.groups = this.convertValues(source["groups"], WatchlistGroup);
	        this.feed = this.convertValues(source["feed"], WatchlistFeed);
	        this.updated_at = source["updated_at"];
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

