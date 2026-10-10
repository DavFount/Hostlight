export namespace main {
	
	export class Target {
	    id: string;
	    name: string;
	    host: string;
	    type: string;
	    port: number;
	    group: string;
	    interval: number;
	    timeout: number;
	    failures: number;
	    recovery: number;
	    state: string;
	    latency: number;
	    message: string;
	    checked: string;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.type = source["type"];
	        this.port = source["port"];
	        this.group = source["group"];
	        this.interval = source["interval"];
	        this.timeout = source["timeout"];
	        this.failures = source["failures"];
	        this.recovery = source["recovery"];
	        this.state = source["state"];
	        this.latency = source["latency"];
	        this.message = source["message"];
	        this.checked = source["checked"];
	    }
	}
	export class BulkRow {
	    line: number;
	    target: Target;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new BulkRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.target = this.convertValues(source["target"], Target);
	        this.error = source["error"];
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
	export class ConfigurationStatus {
	    path: string;
	    error: string;
	    historyError: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigurationStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.error = source["error"];
	        this.historyError = source["historyError"];
	    }
	}
	export class Incident {
	    id: number;
	    targetId: string;
	    name: string;
	    host: string;
	    group: string;
	    type: string;
	    port: number;
	    opened: string;
	    lastSeen: string;
	    ended: string;
	    status: string;
	    reason: string;
	    durationSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new Incident(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.targetId = source["targetId"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.group = source["group"];
	        this.type = source["type"];
	        this.port = source["port"];
	        this.opened = source["opened"];
	        this.lastSeen = source["lastSeen"];
	        this.ended = source["ended"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.durationSeconds = source["durationSeconds"];
	    }
	}
	export class HistoryPage {
	    items: Incident[];
	    total: number;
	    error: string;
	    path: string;
	    retentionDays: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Incident);
	        this.total = source["total"];
	        this.error = source["error"];
	        this.path = source["path"];
	        this.retentionDays = source["retentionDays"];
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
	
	
	export class WebhookDelivery {
	    name: string;
	    host: string;
	    id: string;
	    event: string;
	    created: string;
	    attempts: number;
	    status: string;
	    result: string;
	
	    static createFrom(source: any = {}) {
	        return new WebhookDelivery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.host = source["host"];
	        this.id = source["id"];
	        this.event = source["event"];
	        this.created = source["created"];
	        this.attempts = source["attempts"];
	        this.status = source["status"];
	        this.result = source["result"];
	    }
	}
	export class WebhookSettings {
	    enabled: boolean;
	    configured: boolean;
	    url: string;
	    bearer: string;
	    signingKey: string;
	    allowHTTP: boolean;
	    clearBearer: boolean;
	    clearSigningKey: boolean;
	    hasBearer: boolean;
	    hasSigningKey: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WebhookSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.configured = source["configured"];
	        this.url = source["url"];
	        this.bearer = source["bearer"];
	        this.signingKey = source["signingKey"];
	        this.allowHTTP = source["allowHTTP"];
	        this.clearBearer = source["clearBearer"];
	        this.clearSigningKey = source["clearSigningKey"];
	        this.hasBearer = source["hasBearer"];
	        this.hasSigningKey = source["hasSigningKey"];
	    }
	}

}

