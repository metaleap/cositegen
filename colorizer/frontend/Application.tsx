import React from "react"
import ReactDOM from "react-dom/client"

function Application() {
    return <div>Muh Appl</div>;
}


const domContainer = document.getElementById('uipane')!;
const root = ReactDOM.createRoot(domContainer);
root.render(<Application />);
