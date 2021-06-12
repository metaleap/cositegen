function doPostBack(name) {
    document.getElementById("main_focus_id").value = name;
    document.getElementById("main_form").submit();
}

function toggle(id) {
    const elem = document.getElementById(id);
    elem.style.display = (elem.style.display == 'none') ? 'block' : 'none';
}
