import React, { SyntheticEvent, JSX, useState, useEffect } from "react"
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
    colors[0] = '#ffffff';

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
    const [colorLetter, setColorLetter] = useState(0); // 0-23
    const [colorDigit, setColorDigit] = useState(0); // 0-8
    const onKeyUp = (evt: KeyboardEvent) => {
        const anymod = evt.altKey || evt.ctrlKey || evt.metaKey || evt.shiftKey;
        if ((!anymod) && (evt.key >= 'a') && (evt.key <= 'x'))
            setColorLetter(evt.key.charCodeAt(0) - 'a'.charCodeAt(0));
        else if ((!anymod) && (evt.key >= '1') && (evt.key <= '9'))
            setColorDigit(evt.key.charCodeAt(0) - '1'.charCodeAt(0));
    };
    useEffect(() => {
        document.addEventListener('keyup', onKeyUp);
        return () => { document.removeEventListener('keyup', onKeyUp) };
    }, []);

    return <div className="colr">
        <ColrCanvas />
        <ColrGui colorLetter={colorLetter} colorDigit={colorDigit} />
        <hr />
        <textarea className="dbgJson" spellCheck="false" autoCapitalize="false" autoComplete="false" autoCorrect="false" readOnly={true}>
            {JSON.stringify(ctx, null, "\t")}
        </textarea>
    </div>;
}

function ColrCanvas() {
    return <div className="colrcanvas" dangerouslySetInnerHTML={{ __html: ctx.svgSrc }} />;
}

function ColrGui(props: { colorLetter: number, colorDigit: number }) { // 9*24
    const rows: JSX.Element[] = [];
    for (let i = 0; i < keyedColors.length; i++) {
        const letter = String.fromCharCode('A'.charCodeAt(0) + i);
        const digits = keyedColors[i];
        const cols: JSX.Element[] = [];
        for (let j = 0; j < digits.length; j++) {
            const digit = (j + 1).toString();
            cols.push(<a className={(i === props.colorLetter && j === props.colorDigit) ? "outlinedtext selected" : "outlinedtext"} style={{ backgroundColor: digits[j] }} title={digits[j]}>{letter}{digit}</a>)
        }
        const row: JSX.Element = <div className="colsrow">{cols}</div>
        rows.push(row);
    }

    return <div className="colrgui">
        <label>F: <input type="number" value="0" /></label>
        <label>B: <input type="number" value="0" /></label>
        <hr />
        {rows}
    </div>;
}
