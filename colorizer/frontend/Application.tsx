import React from "react"
import ReactDOM from "react-dom/client"

const domContainer = document.getElementById('uipane')!;
const root = ReactDOM.createRoot(domContainer);
root.render(<Application />);

type ColorizerCtx = {
    bwImgUri: string
    svgSrc: string
}

let ctx: ColorizerCtx = window['__colorizer_ctx'];

function Application() {
    return <div className="colr">
        <div className="colcanvas" /*style={{ backgroundImage: "url(" + ctx.bwImgUri + ")" }}*/ dangerouslySetInnerHTML={{ __html: ctx.svgSrc }} />
        <div className="colgui">
            the GUI
        </div>
    </div>;
}
