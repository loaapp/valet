export namespace models {
	
	export class CreateRouteRequest {
	    domain: string;
	    upstream: string;
	    tls?: boolean;
	    description: string;
	    template: string;
	    templateParams: Record<string, string>;
	    matchConfig: string;
	    handlerConfig: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateRouteRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.upstream = source["upstream"];
	        this.tls = source["tls"];
	        this.description = source["description"];
	        this.template = source["template"];
	        this.templateParams = source["templateParams"];
	        this.matchConfig = source["matchConfig"];
	        this.handlerConfig = source["handlerConfig"];
	    }
	}
	export class DaemonStatus {
	    status: string;
	    routes: number;
	    tlds: number;
	    mkcert: boolean;
	    platform: string;
	
	    static createFrom(source: any = {}) {
	        return new DaemonStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.routes = source["routes"];
	        this.tlds = source["tlds"];
	        this.mkcert = source["mkcert"];
	        this.platform = source["platform"];
	    }
	}
	export class ManagedTLD {
	    tld: string;
	    resolverInstalled: boolean;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedTLD(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tld = source["tld"];
	        this.resolverInstalled = source["resolverInstalled"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class Param {
	    key: string;
	    label: string;
	    placeholder: string;
	    required: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Param(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.placeholder = source["placeholder"];
	        this.required = source["required"];
	    }
	}
	export class Route {
	    id: string;
	    domain: string;
	    upstream: string;
	    tlsEnabled: boolean;
	    certPath: string;
	    keyPath: string;
	    matchConfig: string;
	    handlerConfig: string;
	    template: string;
	    description: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Route(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.domain = source["domain"];
	        this.upstream = source["upstream"];
	        this.tlsEnabled = source["tlsEnabled"];
	        this.certPath = source["certPath"];
	        this.keyPath = source["keyPath"];
	        this.matchConfig = source["matchConfig"];
	        this.handlerConfig = source["handlerConfig"];
	        this.template = source["template"];
	        this.description = source["description"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class Template {
	    slug: string;
	    name: string;
	    description: string;
	    params: Param[];
	
	    static createFrom(source: any = {}) {
	        return new Template(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slug = source["slug"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.params = this.convertValues(source["params"], Param);
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

