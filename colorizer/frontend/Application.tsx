import React, { SyntheticEvent } from "react"
import ReactDOM from "react-dom/client"

const domContainer = document.getElementById('uipane')!;
const root = ReactDOM.createRoot(domContainer);
root.render(<Application />);

type ImgPanel = {
    Rect: { Min: { X: number, Y: number }, Max: { X: number, Y: number } },
    SbBorderOuter: number,
    SbBorderInner: number,
    SubRows: Array<ImgPanel>,
    SubCols: Array<ImgPanel>
};

type SheetVer = {
    ID: string,
    DateTimeUnixNano: number,
    FileName: string,
    Data: {
        DirPath: string,
        BwFilePath: string,
        BwSmallFilePath: string,
        PxCm: number,
        GrayDistr: Array<number>,
        HomePic: string,
        PanelsTree: ImgPanel
    }
};

type ColorizerCtx = {
    numPanels: number,
    sv: SheetVer,
    svgSrc: string
};

let ctx: ColorizerCtx = window['__ctxcolr'];

function Application() {
    return <div className="colr" onLoad={(e: SyntheticEvent<HTMLDivElement>) => { alert(321); }}>
        <ColrCanvas />
        <ColrGui />
        <hr />
        <textarea className="dbgJson" spellCheck="false" autoCapitalize="false" autoComplete="false" autoCorrect="false" readOnly={true}>
            {JSON.stringify(ctx, null, "\t")}
        </textarea>
    </div>;
}

function ColrCanvas() {
    return <div className="colcanvas"
        /*style={{ backgroundImage: "url(" + ctx.bwImgUri + ")" }}*/
        dangerouslySetInnerHTML={{ __html: ctx.svgSrc }} />;
}

function ColrGui() {
    return <div className="colgui">
        the GUI
    </div>;
}
