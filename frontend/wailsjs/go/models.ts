export namespace main {
	
	export class AlertSilence {
	    id: string;
	    targetId: string;
	    kind: string;
	    starts: string;
	    ends: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new AlertSilence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.targetId = source["targetId"];
	        this.kind = source["kind"];
	        this.starts = source["starts"];
	        this.ends = source["ends"];
	        this.reason = source["reason"];
	    }
	}
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
	    expectedStatus?: number;
	    bodyMode?: string;
	    bodyExpected?: string;
	    jsonPointer?: string;
	    jsonType?: string;
	    tlsMinDays?: number;
	    certificateExpires?: string;
	
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
	        this.expectedStatus = source["expectedStatus"];
	        this.bodyMode = source["bodyMode"];
	        this.bodyExpected = source["bodyExpected"];
	        this.jsonPointer = source["jsonPointer"];
	        this.jsonType = source["jsonType"];
	        this.tlsMinDays = source["tlsMinDays"];
	        this.certificateExpires = source["certificateExpires"];
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
	export class CheckStatistics {
	    id: string;
	    checks: number;
	    failed: number;
	    started: string;
	    sessionChecks: number;
	    sessionPassed: number;
	    sessionFailed: number;
	    setupErrors: number;
	
	    static createFrom(source: any = {}) {
	        return new CheckStatistics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.checks = source["checks"];
	        this.failed = source["failed"];
	        this.started = source["started"];
	        this.sessionChecks = source["sessionChecks"];
	        this.sessionPassed = source["sessionPassed"];
	        this.sessionFailed = source["sessionFailed"];
	        this.setupErrors = source["setupErrors"];
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
	export class DesktopStatus {
	    supported: boolean;
	    available: boolean;
	    closeToTray: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DesktopStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supported = source["supported"];
	        this.available = source["available"];
	        this.closeToTray = source["closeToTray"];
	    }
	}
	export class HTTPProbeResult {
	    success: boolean;
	    statusCode: number;
	    latency: number;
	    message: string;
	    timing: string;
	    certificateExpires?: string;
	
	    static createFrom(source: any = {}) {
	        return new HTTPProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.statusCode = source["statusCode"];
	        this.latency = source["latency"];
	        this.message = source["message"];
	        this.timing = source["timing"];
	        this.certificateExpires = source["certificateExpires"];
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
	
	export class PortableMonitor {
	    name: string;
	    host: string;
	    type: string;
	    port: number;
	    group: string;
	    interval: number;
	    timeout: number;
	    failures: number;
	    recovery: number;
	    paused: boolean;
	    expectedStatus?: number;
	    bodyMode?: string;
	    bodyExpected?: string;
	    jsonPointer?: string;
	    jsonType?: string;
	    tlsMinDays?: number;
	
	    static createFrom(source: any = {}) {
	        return new PortableMonitor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.host = source["host"];
	        this.type = source["type"];
	        this.port = source["port"];
	        this.group = source["group"];
	        this.interval = source["interval"];
	        this.timeout = source["timeout"];
	        this.failures = source["failures"];
	        this.recovery = source["recovery"];
	        this.paused = source["paused"];
	        this.expectedStatus = source["expectedStatus"];
	        this.bodyMode = source["bodyMode"];
	        this.bodyExpected = source["bodyExpected"];
	        this.jsonPointer = source["jsonPointer"];
	        this.jsonType = source["jsonType"];
	        this.tlsMinDays = source["tlsMinDays"];
	    }
	}
	export class MonitorImportRow {
	    row: number;
	    monitor: PortableMonitor;
	    status: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new MonitorImportRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.row = source["row"];
	        this.monitor = this.convertValues(source["monitor"], PortableMonitor);
	        this.status = source["status"];
	        this.detail = source["detail"];
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
	export class MonitorImportPreview {
	    token: string;
	    source: string;
	    rows: MonitorImportRow[];
	    ready: number;
	    skipped: number;
	    invalid: number;
	
	    static createFrom(source: any = {}) {
	        return new MonitorImportPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.source = source["source"];
	        this.rows = this.convertValues(source["rows"], MonitorImportRow);
	        this.ready = source["ready"];
	        this.skipped = source["skipped"];
	        this.invalid = source["invalid"];
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
	export class MonitorImportResult {
	    added: number;
	    skipped: number;
	    paused: number;
	
	    static createFrom(source: any = {}) {
	        return new MonitorImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.skipped = source["skipped"];
	        this.paused = source["paused"];
	    }
	}
	
	
	export class SchedulingSettings {
	    concurrency: number;
	    spreadMilliseconds: number;
	
	    static createFrom(source: any = {}) {
	        return new SchedulingSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.concurrency = source["concurrency"];
	        this.spreadMilliseconds = source["spreadMilliseconds"];
	    }
	}
	export class SchedulingStatus {
	    saved: SchedulingSettings;
	    active: SchedulingSettings;
	    restartRequired: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SchedulingStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saved = this.convertValues(source["saved"], SchedulingSettings);
	        this.active = this.convertValues(source["active"], SchedulingSettings);
	        this.restartRequired = source["restartRequired"];
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
	export class SupportReport {
	    format: string;
	    version: number;
	    created: string;
	    release: string;
	    os: string;
	    architecture: string;
	    goVersion: string;
	    wailsVersion: string;
	    configurationHealth: string;
	    historyHealth: string;
	    storageSummary: string;
	    monitorCount: number;
	    states: Record<string, number>;
	    checks: Record<string, number>;
	    openIncidents: number;
	    pendingDeliveries: number;
	    probeConcurrency: number;
	    historyQueueLength: number;
	    historyQueueCapacity: number;
	
	    static createFrom(source: any = {}) {
	        return new SupportReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.version = source["version"];
	        this.created = source["created"];
	        this.release = source["release"];
	        this.os = source["os"];
	        this.architecture = source["architecture"];
	        this.goVersion = source["goVersion"];
	        this.wailsVersion = source["wailsVersion"];
	        this.configurationHealth = source["configurationHealth"];
	        this.historyHealth = source["historyHealth"];
	        this.storageSummary = source["storageSummary"];
	        this.monitorCount = source["monitorCount"];
	        this.states = source["states"];
	        this.checks = source["checks"];
	        this.openIncidents = source["openIncidents"];
	        this.pendingDeliveries = source["pendingDeliveries"];
	        this.probeConcurrency = source["probeConcurrency"];
	        this.historyQueueLength = source["historyQueueLength"];
	        this.historyQueueCapacity = source["historyQueueCapacity"];
	    }
	}
	
	export class VisualAlert {
	    sequence: number;
	    name: string;
	    event: string;
	    time: string;
	    host: string;
	    reason: string;
	    durationSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new VisualAlert(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sequence = source["sequence"];
	        this.name = source["name"];
	        this.event = source["event"];
	        this.time = source["time"];
	        this.host = source["host"];
	        this.reason = source["reason"];
	        this.durationSeconds = source["durationSeconds"];
	    }
	}
	export class VisualAlertPage {
	    cursor: number;
	    items: VisualAlert[];
	    dropped: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VisualAlertPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cursor = source["cursor"];
	        this.items = this.convertValues(source["items"], VisualAlert);
	        this.dropped = source["dropped"];
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
	    destinationId: string;
	    destinationName: string;
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
	        this.destinationId = source["destinationId"];
	        this.destinationName = source["destinationName"];
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
	    error?: string;
	    id: string;
	    name: string;
	    scope: string;
	    groups: string[];
	    monitorIds: string[];
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
	        this.error = source["error"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scope = source["scope"];
	        this.groups = source["groups"];
	        this.monitorIds = source["monitorIds"];
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

