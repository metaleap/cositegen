import React from "react";
import ReactDOM from "react-dom/client";

export default function Colorizer() {
    return (
        <div>DaColorizer</div>
    );
}

const domContainer = document.querySelector('#colorizer')!;
const root = ReactDOM.createRoot(domContainer);
root.render(<Colorizer />);
