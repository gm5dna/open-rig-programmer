export namespace codeplug {
	
	export class BoolField {
	    state: string;
	    value?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BoolField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.value = source["value"];
	    }
	}
	export class ToneField {
	    state: string;
	    value?: number;
	
	    static createFrom(source: any = {}) {
	        return new ToneField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.value = source["value"];
	    }
	}
	export class ChannelData {
	    freq_hz: number;
	    mode: string;
	    clar_hz?: number;
	    rx_clar?: boolean;
	    tx_clar?: boolean;
	    ctcss: string;
	    ctcss_tone: ToneField;
	    shift: string;
	    tag?: string;
	    tag_display: BoolField;
	    scan_skip: BoolField;
	
	    static createFrom(source: any = {}) {
	        return new ChannelData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.freq_hz = source["freq_hz"];
	        this.mode = source["mode"];
	        this.clar_hz = source["clar_hz"];
	        this.rx_clar = source["rx_clar"];
	        this.tx_clar = source["tx_clar"];
	        this.ctcss = source["ctcss"];
	        this.ctcss_tone = this.convertValues(source["ctcss_tone"], ToneField);
	        this.shift = source["shift"];
	        this.tag = source["tag"];
	        this.tag_display = this.convertValues(source["tag_display"], BoolField);
	        this.scan_skip = this.convertValues(source["scan_skip"], BoolField);
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
	export class Channel {
	    slot: string;
	    data?: ChannelData;
	
	    static createFrom(source: any = {}) {
	        return new Channel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slot = source["slot"];
	        this.data = this.convertValues(source["data"], ChannelData);
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
	
	export class RadioInfo {
	    model: string;
	    cat_id: string;
	    // Go type: time
	    read_at: any;
	    port?: string;
	    usb_serial?: string;
	    firmware_confirmed?: string;
	    region?: string;
	    baseline_digest?: string;
	
	    static createFrom(source: any = {}) {
	        return new RadioInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.cat_id = source["cat_id"];
	        this.read_at = this.convertValues(source["read_at"], null);
	        this.port = source["port"];
	        this.usb_serial = source["usb_serial"];
	        this.firmware_confirmed = source["firmware_confirmed"];
	        this.region = source["region"];
	        this.baseline_digest = source["baseline_digest"];
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
	
	export class SlotView {
	    Slot: string;
	    Display: string;
	
	    static createFrom(source: any = {}) {
	        return new SlotView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Slot = source["Slot"];
	        this.Display = source["Display"];
	    }
	}
	export class BankView {
	    ID: string;
	    Label: string;
	    ReadOnly: boolean;
	    Slots: SlotView[];
	
	    static createFrom(source: any = {}) {
	        return new BankView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Label = source["Label"];
	        this.ReadOnly = source["ReadOnly"];
	        this.Slots = this.convertValues(source["Slots"], SlotView);
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
	export class CodeplugView {
	    Schema: number;
	    Generator: string;
	    Radio: codeplug.RadioInfo;
	    Channels: codeplug.Channel[];
	    WorkingPath: string;
	    Dirty: boolean;
	    BaselineStale: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CodeplugView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Schema = source["Schema"];
	        this.Generator = source["Generator"];
	        this.Radio = this.convertValues(source["Radio"], codeplug.RadioInfo);
	        this.Channels = this.convertValues(source["Channels"], codeplug.Channel);
	        this.WorkingPath = source["WorkingPath"];
	        this.Dirty = source["Dirty"];
	        this.BaselineStale = source["BaselineStale"];
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
	export class ConnectionInfo {
	    Model: string;
	    CATID: string;
	    Port: string;
	    USBSerial: string;
	    Region: string;
	    Demo: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Model = source["Model"];
	        this.CATID = source["CATID"];
	        this.Port = source["Port"];
	        this.USBSerial = source["USBSerial"];
	        this.Region = source["Region"];
	        this.Demo = source["Demo"];
	    }
	}
	export class DiffCounts {
	    Added: number;
	    Modified: number;
	    Erased: number;
	    Blocked: number;
	    Unchanged: number;
	
	    static createFrom(source: any = {}) {
	        return new DiffCounts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Added = source["Added"];
	        this.Modified = source["Modified"];
	        this.Erased = source["Erased"];
	        this.Blocked = source["Blocked"];
	        this.Unchanged = source["Unchanged"];
	    }
	}
	export class DiffEntryView {
	    Slot: string;
	    SlotDisplay: string;
	    Bank: string;
	    Kind: string;
	    Before?: codeplug.ChannelData;
	    After?: codeplug.ChannelData;
	    Blocked: boolean;
	    BlockReason: string;
	
	    static createFrom(source: any = {}) {
	        return new DiffEntryView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Slot = source["Slot"];
	        this.SlotDisplay = source["SlotDisplay"];
	        this.Bank = source["Bank"];
	        this.Kind = source["Kind"];
	        this.Before = this.convertValues(source["Before"], codeplug.ChannelData);
	        this.After = this.convertValues(source["After"], codeplug.ChannelData);
	        this.Blocked = source["Blocked"];
	        this.BlockReason = source["BlockReason"];
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
	export class DiffSummaryView {
	    Added: DiffEntryView[];
	    Modified: DiffEntryView[];
	    Erased: DiffEntryView[];
	    Counts: DiffCounts;
	
	    static createFrom(source: any = {}) {
	        return new DiffSummaryView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Added = this.convertValues(source["Added"], DiffEntryView);
	        this.Modified = this.convertValues(source["Modified"], DiffEntryView);
	        this.Erased = this.convertValues(source["Erased"], DiffEntryView);
	        this.Counts = this.convertValues(source["Counts"], DiffCounts);
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
	export class DiffView {
	    Diff: DiffSummaryView;
	
	    static createFrom(source: any = {}) {
	        return new DiffView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Diff = this.convertValues(source["Diff"], DiffSummaryView);
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
	export class IssueView {
	    Slot: string;
	    Field: string;
	    Severity: string;
	    Msg: string;
	
	    static createFrom(source: any = {}) {
	        return new IssueView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Slot = source["Slot"];
	        this.Field = source["Field"];
	        this.Severity = source["Severity"];
	        this.Msg = source["Msg"];
	    }
	}
	export class EditResult {
	    Issues: IssueView[];
	    Dirty: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EditResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Issues = this.convertValues(source["Issues"], IssueView);
	        this.Dirty = source["Dirty"];
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
	export class LossEntryView {
	    Line: number;
	    Column: string;
	    Value: string;
	    Action: string;
	    Detail: string;
	    Blocking: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LossEntryView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Line = source["Line"];
	        this.Column = source["Column"];
	        this.Value = source["Value"];
	        this.Action = source["Action"];
	        this.Detail = source["Detail"];
	        this.Blocking = source["Blocking"];
	    }
	}
	export class ImportResultView {
	    Path: string;
	    Cancelled: boolean;
	    ParseError: string;
	    LossEntries: LossEntryView[];
	    HasBlockingLoss: boolean;
	    RefusalReason: string;
	    Merged: boolean;
	    Issues: IssueView[];
	    Dirty: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImportResultView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Path = source["Path"];
	        this.Cancelled = source["Cancelled"];
	        this.ParseError = source["ParseError"];
	        this.LossEntries = this.convertValues(source["LossEntries"], LossEntryView);
	        this.HasBlockingLoss = source["HasBlockingLoss"];
	        this.RefusalReason = source["RefusalReason"];
	        this.Merged = source["Merged"];
	        this.Issues = this.convertValues(source["Issues"], IssueView);
	        this.Dirty = source["Dirty"];
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
	
	
	export class PortEntry {
	    Path: string;
	    Description: string;
	    Score: number;
	    Hints: string[];
	
	    static createFrom(source: any = {}) {
	        return new PortEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Path = source["Path"];
	        this.Description = source["Description"];
	        this.Score = source["Score"];
	        this.Hints = source["Hints"];
	    }
	}
	export class PreservationTooltipsView {
	    Tone: string;
	    ScanSkip: string;
	
	    static createFrom(source: any = {}) {
	        return new PreservationTooltipsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Tone = source["Tone"];
	        this.ScanSkip = source["ScanSkip"];
	    }
	}
	export class SendPlanView {
	    Diff: DiffSummaryView;
	    SnapshotPath: string;
	    BaselineDigestShort: string;
	    ConfirmationDigest: string;
	    NothingToSend: boolean;
	    FirmwareRequired: boolean;
	    FirmwareGuidance: string;
	
	    static createFrom(source: any = {}) {
	        return new SendPlanView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Diff = this.convertValues(source["Diff"], DiffSummaryView);
	        this.SnapshotPath = source["SnapshotPath"];
	        this.BaselineDigestShort = source["BaselineDigestShort"];
	        this.ConfirmationDigest = source["ConfirmationDigest"];
	        this.NothingToSend = source["NothingToSend"];
	        this.FirmwareRequired = source["FirmwareRequired"];
	        this.FirmwareGuidance = source["FirmwareGuidance"];
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
	export class SettingEntryView {
	    ID: string;
	    Value: string;
	    State: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingEntryView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Value = source["Value"];
	        this.State = source["State"];
	    }
	}
	export class SettingItemView {
	    ID: string;
	    Label: string;
	    Display: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingItemView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Label = source["Label"];
	        this.Display = source["Display"];
	    }
	}
	export class SettingGroupView {
	    ID: string;
	    Label: string;
	    Items: SettingItemView[];
	
	    static createFrom(source: any = {}) {
	        return new SettingGroupView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Label = source["Label"];
	        this.Items = this.convertValues(source["Items"], SettingItemView);
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
	
	export class SettingMenuView {
	    ID: string;
	    Label: string;
	    Groups: SettingGroupView[];
	
	    static createFrom(source: any = {}) {
	        return new SettingMenuView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Label = source["Label"];
	        this.Groups = this.convertValues(source["Groups"], SettingGroupView);
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
	export class SettingsSpecView {
	    Live: boolean;
	    DescriptorVersion: string;
	    Menus: SettingMenuView[];
	
	    static createFrom(source: any = {}) {
	        return new SettingsSpecView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Live = source["Live"];
	        this.DescriptorVersion = source["DescriptorVersion"];
	        this.Menus = this.convertValues(source["Menus"], SettingMenuView);
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
	export class SettingsView {
	    HasSnapshot: boolean;
	    Descriptor: string;
	    Complete: boolean;
	    HasLegacy: boolean;
	    Entries: SettingEntryView[];
	
	    static createFrom(source: any = {}) {
	        return new SettingsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.HasSnapshot = source["HasSnapshot"];
	        this.Descriptor = source["Descriptor"];
	        this.Complete = source["Complete"];
	        this.HasLegacy = source["HasLegacy"];
	        this.Entries = this.convertValues(source["Entries"], SettingEntryView);
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
	
	export class ToneView {
	    Decihertz: number;
	    Display: string;
	
	    static createFrom(source: any = {}) {
	        return new ToneView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Decihertz = source["Decihertz"];
	        this.Display = source["Display"];
	    }
	}
	export class UISpecView {
	    Live: boolean;
	    Banks: BankView[];
	    Modes: string[];
	    ShiftOptions: string[];
	    CTCSSStateOptions: string[];
	    Tones: ToneView[];
	    TagMaxBytes: number;
	    ClarMaxHz: number;
	    ClarStepHz: number;
	    ToneScanSkipNote: string;
	    ToneScanSkipVerification: string;
	    EraseDialogNote: string;
	    PreservationTooltips: PreservationTooltipsView;
	    FirmwarePlaceholder: string;
	
	    static createFrom(source: any = {}) {
	        return new UISpecView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Live = source["Live"];
	        this.Banks = this.convertValues(source["Banks"], BankView);
	        this.Modes = source["Modes"];
	        this.ShiftOptions = source["ShiftOptions"];
	        this.CTCSSStateOptions = source["CTCSSStateOptions"];
	        this.Tones = this.convertValues(source["Tones"], ToneView);
	        this.TagMaxBytes = source["TagMaxBytes"];
	        this.ClarMaxHz = source["ClarMaxHz"];
	        this.ClarStepHz = source["ClarStepHz"];
	        this.ToneScanSkipNote = source["ToneScanSkipNote"];
	        this.ToneScanSkipVerification = source["ToneScanSkipVerification"];
	        this.EraseDialogNote = source["EraseDialogNote"];
	        this.PreservationTooltips = this.convertValues(source["PreservationTooltips"], PreservationTooltipsView);
	        this.FirmwarePlaceholder = source["FirmwarePlaceholder"];
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
	export class ValidationView {
	    Issues: IssueView[];
	    Advisory: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ValidationView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Issues = this.convertValues(source["Issues"], IssueView);
	        this.Advisory = source["Advisory"];
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
	export class VersionView {
	    Version: string;
	    Display: string;
	    IsRelease: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VersionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Version = source["Version"];
	        this.Display = source["Display"];
	        this.IsRelease = source["IsRelease"];
	    }
	}

}

