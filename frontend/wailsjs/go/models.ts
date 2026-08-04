export namespace core {
	
	export class BrowserAgent {
	    Name: string;
	    SystemPrompt: string;
	    Workspace: string;
	    TargetURL: string;
	    PermissionMode: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserAgent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.SystemPrompt = source["SystemPrompt"];
	        this.Workspace = source["Workspace"];
	        this.TargetURL = source["TargetURL"];
	        this.PermissionMode = source["PermissionMode"];
	    }
	}

}

export namespace db {
	
	export class Message {
	    id: string;
	    session_id: string;
	    role: string;
	    content: string;
	    thinking: string;
	    tool_calls: string;
	    timestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.session_id = source["session_id"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.thinking = source["thinking"];
	        this.tool_calls = source["tool_calls"];
	        this.timestamp = source["timestamp"];
	    }
	}
	export class Session {
	    id: string;
	    title: string;
	    workspace: string;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.workspace = source["workspace"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}

}

export namespace llm {
	
	export class ExtraFieldDef {
	    Key: string;
	    Label: string;
	    Placeholder: string;
	    EnvVar: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtraFieldDef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Key = source["Key"];
	        this.Label = source["Label"];
	        this.Placeholder = source["Placeholder"];
	        this.EnvVar = source["EnvVar"];
	    }
	}
	export class ModelConfig {
	    ID: string;
	    Label: string;
	    BaseURL: string;
	    APIKey: string;
	    Model: string;
	    BackendType: string;
	    AuthType: string;
	    Headers: Record<string, string>;
	    CompletionsURL: string;
	    ExtraBody: Record<string, any>;
	    BuiltIn: boolean;
	    EnvVarName: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Label = source["Label"];
	        this.BaseURL = source["BaseURL"];
	        this.APIKey = source["APIKey"];
	        this.Model = source["Model"];
	        this.BackendType = source["BackendType"];
	        this.AuthType = source["AuthType"];
	        this.Headers = source["Headers"];
	        this.CompletionsURL = source["CompletionsURL"];
	        this.ExtraBody = source["ExtraBody"];
	        this.BuiltIn = source["BuiltIn"];
	        this.EnvVarName = source["EnvVarName"];
	    }
	}
	export class ProviderConfig {
	    ID: string;
	    Name: string;
	    BaseURL: string;
	    CompletionsURL: string;
	    APIKey: string;
	    AuthType: string;
	    HeaderName: string;
	    Headers: Record<string, string>;
	    Models: string[];
	    ExtraFields: Record<string, string>;
	    AutoDetect: boolean;
	    EnvVar: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.BaseURL = source["BaseURL"];
	        this.CompletionsURL = source["CompletionsURL"];
	        this.APIKey = source["APIKey"];
	        this.AuthType = source["AuthType"];
	        this.HeaderName = source["HeaderName"];
	        this.Headers = source["Headers"];
	        this.Models = source["Models"];
	        this.ExtraFields = source["ExtraFields"];
	        this.AutoDetect = source["AutoDetect"];
	        this.EnvVar = source["EnvVar"];
	    }
	}
	export class ProviderPreset {
	    Name: string;
	    Icon: string;
	    BaseURL: string;
	    Model: string;
	    AuthType: string;
	    HeaderName: string;
	    EnvVar: string;
	    Description: string;
	    Models: string[];
	    ExtraFields: ExtraFieldDef[];
	    AutoDetect: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProviderPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Icon = source["Icon"];
	        this.BaseURL = source["BaseURL"];
	        this.Model = source["Model"];
	        this.AuthType = source["AuthType"];
	        this.HeaderName = source["HeaderName"];
	        this.EnvVar = source["EnvVar"];
	        this.Description = source["Description"];
	        this.Models = source["Models"];
	        this.ExtraFields = this.convertValues(source["ExtraFields"], ExtraFieldDef);
	        this.AutoDetect = source["AutoDetect"];
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

