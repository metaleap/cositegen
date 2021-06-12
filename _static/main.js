function doPostBack(name) {
    document.getElementById("main_focus_id").value = name;
    document.getElementById("main_form").submit();
}

function refreshPanelRects(numPanels, maxImagePanelAreas) {
    try {
        for (let i = 0; i < numPanels; i++) {
            const pid = "p" + i;
            const span = document.getElementById(pid + 'rects');
            span.innerHTML = "";
            for (let j = 0; j < maxImagePanelAreas; j++) {
                const trx0 = parseInt(document.getElementById(pid + "t" + j + "rx0").value);
                const trx1 = parseInt(document.getElementById(pid + "t" + j + "rx1").value);
                const try0 = parseInt(document.getElementById(pid + "t" + j + "ry0").value);
                const try1 = parseInt(document.getElementById(pid + "t" + j + "ry1").value);
                const width = trx1 - trx0;
                const height = try1 - try0;
                if ((!(isNaN(width) || isNaN(height))) && width > 0 && height > 0) {
                    span.innerHTML += "<div class='panelrect col" + j + "' style='left:" + trx0 + "px; top:" + try0 + "px; width: " + width + "px; height: " + height + "px;'>panel rect " + j + "</div>";
                }
            }
        }
    } catch (e) {
        alert(e);
    }
}

function toggle(id) {
    const elem = document.getElementById(id);
    elem.style.display = (elem.style.display == 'none') ? 'block' : 'none';
}
