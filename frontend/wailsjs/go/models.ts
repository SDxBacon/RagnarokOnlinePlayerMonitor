export namespace config {
	
	export class LoginServer {
	    Name: string;
	    IP: string;
	    Port: number;
	    Pattern: number[];
	    IsNumberResponse: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LoginServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.IP = source["IP"];
	        this.Port = source["Port"];
	        this.Pattern = source["Pattern"];
	        this.IsNumberResponse = source["IsNumberResponse"];
	    }
	}

}

export namespace ragnarok {
	
	export class CharacterServerInfo {
	    Name: string;
	    Url: string;
	    Players: number;
	
	    static createFrom(source: any = {}) {
	        return new CharacterServerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Url = source["Url"];
	        this.Players = source["Players"];
	    }
	}

}

