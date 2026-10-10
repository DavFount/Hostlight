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
	
	    static createFrom(source: any = {}) {
	        return new ConfigurationStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.error = source["error"];
	    }
	}

}

