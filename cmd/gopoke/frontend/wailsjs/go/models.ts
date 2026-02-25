export namespace execution {
	
	export class Diagnostic {
	    Kind: string;
	    File: string;
	    Line: number;
	    Column: number;
	    Message: string;
	
	    static createFrom(source: any = {}) {
	        return new Diagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Kind = source["Kind"];
	        this.File = source["File"];
	        this.Line = source["Line"];
	        this.Column = source["Column"];
	        this.Message = source["Message"];
	    }
	}
	export class Result {
	    Stdout: string;
	    Stderr: string;
	    ExitCode: number;
	    DurationMS: number;
	    TimedOut: boolean;
	    Canceled: boolean;
	    StdoutTruncated: boolean;
	    StderrTruncated: boolean;
	    Diagnostics: Diagnostic[];
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Stdout = source["Stdout"];
	        this.Stderr = source["Stderr"];
	        this.ExitCode = source["ExitCode"];
	        this.DurationMS = source["DurationMS"];
	        this.TimedOut = source["TimedOut"];
	        this.Canceled = source["Canceled"];
	        this.StdoutTruncated = source["StdoutTruncated"];
	        this.StderrTruncated = source["StderrTruncated"];
	        this.Diagnostics = this.convertValues(source["Diagnostics"], Diagnostic);
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
	export class RunRequest {
	    runId: string;
	    projectPath: string;
	    packagePath: string;
	    source: string;
	    timeoutMs: number;
	
	    static createFrom(source: any = {}) {
	        return new RunRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.projectPath = source["projectPath"];
	        this.packagePath = source["packagePath"];
	        this.source = source["source"];
	        this.timeoutMs = source["timeoutMs"];
	    }
	}

}

export namespace project {
	
	export class ModuleInfo {
	    Path: string;
	    ModuleFile: string;
	    HasModule: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModuleInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Path = source["Path"];
	        this.ModuleFile = source["ModuleFile"];
	        this.HasModule = source["HasModule"];
	    }
	}
	export class RunTarget {
	    Package: string;
	    Command: string;
	    Path: string;
	
	    static createFrom(source: any = {}) {
	        return new RunTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Package = source["Package"];
	        this.Command = source["Command"];
	        this.Path = source["Path"];
	    }
	}
	export class OpenProjectResult {
	    Project: storage.ProjectRecord;
	    Module: ModuleInfo;
	    Targets: RunTarget[];
	    EnvVars: storage.EnvVarRecord[];
	    EnvLoadWarnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new OpenProjectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Project = this.convertValues(source["Project"], storage.ProjectRecord);
	        this.Module = this.convertValues(source["Module"], ModuleInfo);
	        this.Targets = this.convertValues(source["Targets"], RunTarget);
	        this.EnvVars = this.convertValues(source["EnvVars"], storage.EnvVarRecord);
	        this.EnvLoadWarnings = source["EnvLoadWarnings"];
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
	
	export class ToolchainInfo {
	    name: string;
	    path: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolchainInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.version = source["version"];
	    }
	}

}

export namespace runner {
	
	export class Worker {
	    ProjectPath: string;
	    // Go type: time
	    StartedAt: any;
	    PID: number;
	    Running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Worker(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ProjectPath = source["ProjectPath"];
	        this.StartedAt = this.convertValues(source["StartedAt"], null);
	        this.PID = source["PID"];
	        this.Running = source["Running"];
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

export namespace storage {
	
	export class EnvVarRecord {
	    id: string;
	    projectId: string;
	    key: string;
	    value: string;
	    masked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EnvVarRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.key = source["key"];
	        this.value = source["value"];
	        this.masked = source["masked"];
	    }
	}
	export class HealthReport {
	    Ready: boolean;
	    Path: string;
	    SchemaVersion: number;
	
	    static createFrom(source: any = {}) {
	        return new HealthReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Ready = source["Ready"];
	        this.Path = source["Path"];
	        this.SchemaVersion = source["SchemaVersion"];
	    }
	}
	export class ProjectRecord {
	    id: string;
	    path: string;
	    // Go type: time
	    lastOpenedAt: any;
	    defaultPackage: string;
	    workingDirectory: string;
	    toolchain: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.lastOpenedAt = this.convertValues(source["lastOpenedAt"], null);
	        this.defaultPackage = source["defaultPackage"];
	        this.workingDirectory = source["workingDirectory"];
	        this.toolchain = source["toolchain"];
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
	export class SnippetRecord {
	    id: string;
	    projectId: string;
	    name: string;
	    content: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SnippetRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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

