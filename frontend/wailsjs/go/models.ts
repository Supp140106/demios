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
	
	export class ModelConfig {
	    ID: string;
	    Label: string;
	    BaseURL: string;
	    APIKey: string;
	    Model: string;
	    BackendType: string;
	    ExtraBody: Record<string, any>;
	
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
	        this.ExtraBody = source["ExtraBody"];
	    }
	}

}

