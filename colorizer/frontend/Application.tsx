import React, { SyntheticEvent, JSX } from "react"
import ReactDOM from "react-dom/client"

const domContainer = document.getElementById('uipane')!;
const root = ReactDOM.createRoot(domContainer);
root.render(<Application />);

type ImgPanel = {
    Rect: { Min: { X: number, Y: number }, Max: { X: number, Y: number } },
    SbBorderOuter: number,
    SbBorderInner: number,
    SubRows: ImgPanel[],
    SubCols: ImgPanel[]
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
        GrayDistr: number[],
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
let keyedColors: string[][] = [];

function init() {
    const n = ['33', '55', '88', 'AA', 'CC', 'EE'];
    const colors: string[] = [];
    for (let r of n) for (let g of n) for (let b of n)
        colors.push('#' + r + g + b);
    let idx_color = 0;

    for (let letter = 1; letter <= 24; letter++) {
        let digs: string[] = [];
        for (let digit = 1; digit <= 9; digit++) {
            digs.push(colors[idx_color]);
            idx_color++;
        }
        keyedColors.push(digs);
    }

}

init();

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
    return <div className="colrcanvas"
        /*style={{ backgroundImage: "url(" + ctx.bwImgUri + ")" }}*/
        dangerouslySetInnerHTML={{ __html: ctx.svgSrc }} />;
}

function ColrGui() { // 9*24
    const rows: JSX.Element[] = [];
    for (let i = 0; i < keyedColors.length; i++) {
        const letter = String.fromCharCode('A'.charCodeAt(0) + i);
        const digits = keyedColors[i];
        const cols: JSX.Element[] = [];
        for (let j = 0; j < digits.length; j++) {
            const digit = (j + 1).toString();
            cols.push(<div className="outlined" style={{ backgroundColor: digits[j] }} title={digits[j]}>{letter}{digit}</div>)
        }
        const row: JSX.Element = <div className="colsrow">{cols}</div>
        rows.push(row);
    }

    return <div className="colrgui">
        the GUI<hr />
        {rows}
    </div>;
}
