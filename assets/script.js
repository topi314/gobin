hljs.listLanguages().forEach((language) => {
    const option = document.createElement("option");
    option.value = language;
    option.innerText = language;

    const languageElement = document.querySelector("#language")
    if (languageElement.value === language) {
        languageElement.removeChild(languageElement.querySelector(`option[value="${language}"]`));
        option.selected = true;
    }

    languageElement.appendChild(option);
});

document.addEventListener("DOMContentLoaded", () => {
    const key = window.location.pathname === "/" ? "" : window.location.pathname.slice(1);
    const params = new URLSearchParams(window.location.search);
    if (params.has("token")) {
        setDocumentToken(key, params.get("token"));
    }

    let newState;
    let url;
    if (key) {
        const content = document.querySelector("#code-view").innerText
        const language = document.querySelector("#language").value;
        newState = {key: key, mode: "view", content: content, language: language};
        url = `/${key}`;
    } else {
        newState = {key: "", mode: "edit", content: "", language: ""};
        url = "/";
    }

    updateCode(newState);
    updatePage(newState);

    window.history.replaceState(newState, "", url);
});

window.addEventListener("popstate", (event) => {
    updateCode(event.state);
    updatePage(event.state);
});

document.querySelector("#code-edit").addEventListener("keydown", (event) => {
    if (event.key !== "Tab" || event.shiftKey) {
        return;
    }
    event.preventDefault();

    const start = event.target.selectionStart;
    const end = event.target.selectionEnd;
    event.target.value = event.target.value.substring(0, start) + "\t" + event.target.value.substring(end);
    event.target.selectionStart = event.target.selectionEnd = start + 1;
})

document.addEventListener("keydown", (event) => {
    if (!event.ctrlKey) return;

    switch (event.key) {
        case "s":
            event.preventDefault();
            if (document.querySelector("#save").disabled) return;
            document.querySelector("#save").click();
            break;
        case "n":
            event.preventDefault();
            document.querySelector("#new").click();
            break;
        case "e":
            event.preventDefault();
            if (document.querySelector("#edit").disabled) return;
            document.querySelector("#edit").click();
            break;
        case "d":
            event.preventDefault();
            if (document.querySelector("#delete").disabled) return;
            document.querySelector("#delete").click();
            break;
    }
})

document.querySelector("#code-edit").addEventListener("keyup", (event) => {
    const {key, language} = getState();
    const newState = {key: key, mode: "edit", content: event.target.value, language: language};
    window.history.replaceState(newState, "", `/${key}`);
    updatePage(newState);
})

document.querySelector("#edit").addEventListener("click", async () => {
    if (document.querySelector("#edit").disabled) return;

    const {key, content, language} = getState();
    let newState;
    let url;
    if (getDocumentToken(key) === "") {
        newState = {key: "", mode: "edit", content: content, language: language};
        url = "/";
    } else {
        newState = {key: key, mode: "edit", content: content, language: language};
        url = `/${key}`;
    }

    updateCode(newState);
    updatePage(newState);

    window.history.pushState(newState, "", url);
})

document.querySelector("#save").addEventListener("click", async () => {
    if (document.querySelector("#save").disabled) return;

    const {key, mode, content, language} = getState()
    if (mode !== "edit") return;
    const token = getDocumentToken(key);
    const saveButton = document.querySelector("#save");
    saveButton.classList.add("loading");

    let response;
    if (key && token) {
        response = await fetch(`/documents/${key}`, {
            method: "PATCH",
            body: content,
            headers: {
                Authorization: `Bearer ${token}`,
                Language: language
            }
        });
    } else {
        response = await fetch("/documents", {
            method: "POST",
            body: content,
            headers: {
                Language: language
            }
        });
    }
    saveButton.classList.remove("loading");

    const body = await response.json();
    if (!response.ok) {
        showErrorPopup(body.message || response.statusText);
        console.error("error saving document:", response);
        return;
    }

    const newState = {key: body.key, mode: "view", content: body.data, language: body.language};
    setDocumentToken(body.key, body.token);
    updateCode(newState);
    updatePage(newState);
    window.history.pushState(newState, "", `/${body.key}`);
});

document.querySelector("#delete").addEventListener("click", async () => {
    if (document.querySelector("#delete").disabled) return;

    const {key} = getState();
    const token = getDocumentToken(key);
    if (token === "") {
        return;
    }

    const deleteConfirm = window.confirm("Are you sure you want to delete this document? This action cannot be undone.")
    if (!deleteConfirm) return;

    const deleteButton = document.querySelector("#delete");
    deleteButton.classList.add("loading");
    let response = await fetch(`/documents/${key}`, {
        method: "DELETE",
        headers: {
            Authorization: `Bearer ${token}`
        }
    });
    deleteButton.classList.remove("loading");

    if (!response.ok) {
        const body = await response.json();
        showErrorPopup(body.message || response.statusText)
        console.error("error deleting document:", response);
        return;
    }
    deleteToken();
    const newState = {key: "", mode: "edit", content: "", language: ""};
    updateCode(newState);
    updatePage(newState);
    window.history.pushState(newState, "", "/");
})

document.querySelector("#copy").addEventListener("click", async () => {
    if (document.querySelector("#copy").disabled) return;

    const {content} = getState();
    if (!content) return;
    await navigator.clipboard.writeText(content);
})

document.querySelector("#raw").addEventListener("click", () => {
    if (document.querySelector("#raw").disabled) return;

    const {key} = getState();
    if (!key) return;
    window.open(`/raw/${key}`, "_blank").focus();
})

document.querySelector("#share").addEventListener("click", async () => {
    if (document.querySelector("#share").disabled) return;

    const {key} = getState();
    const token = getDocumentToken(key);
    if (!token) {
        await navigator.clipboard.writeText(window.location.href);
        return;
    }

    document.querySelector("#share-permissions-write").checked = false;
    document.querySelector("#share-permissions-delete").checked = false;
    document.querySelector("#share-permissions-share").checked = false;

    document.querySelector("#share-dialog").showModal();
});

document.querySelector("#share-dialog-close").addEventListener("click", () => {
    document.querySelector("#share-dialog").close();
});

document.querySelector("#share-copy").addEventListener("click", async () => {
    const permissions = [];
    if (document.querySelector("#share-permissions-write").checked) {
        permissions.push("write");
    }
    if (document.querySelector("#share-permissions-delete").checked) {
        permissions.push("delete");
    }
    if (document.querySelector("#share-permissions-share").checked) {
        permissions.push("share");
    }

    if (permissions.length === 0) {
        await navigator.clipboard.writeText(window.location.href);
        document.querySelector("#share-dialog").close();
        return;
    }

    const {key} = getState();
    const token = getDocumentToken(key);

    const response = await fetch(`/documents/${key}/share`, {
        method: "POST",
        body: JSON.stringify({permissions: permissions}),
        headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`
        }
    });

    if (!response.ok) {
        const body = await response.json();
        showErrorPopup(body.message || response.statusText)
        console.error("error sharing document:", response);
        return;
    }

    const body = await response.json()
    const shareUrl = window.location.href + "?token=" + body.token;
    await navigator.clipboard.writeText(shareUrl);
    document.querySelector("#share-dialog").close();
});


document.querySelector("#language").addEventListener("change", (event) => {
    const {key, mode, content} = getState();
    const newState = {key: key, mode: mode, content: content, language: event.target.value};
    highlightCode(newState);
    window.history.replaceState(newState, "", window.location.pathname);
});

document.querySelector("#style").addEventListener("change", (event) => {
    setStyle(event.target.value);
});

function showErrorPopup(message) {
    const popup = document.getElementById("error-popup");
    popup.style.display = "block";
    popup.innerText = message || "Something went wrong.";
    setTimeout(() => popup.style.display = "none", 5000);
}


function getState() {
    return window.history.state;
}

function getDocumentToken(key) {
    const documents = localStorage.getItem("documents")
    if (!documents) return ""
    const token = JSON.parse(documents)[key]
    if (!token) return ""

    return token
}

function setDocumentToken(key, token) {
    let documents = localStorage.getItem("documents")
    if (!documents) {
        documents = "{}"
    }
    const parsedDocuments = JSON.parse(documents)
    parsedDocuments[key] = token
    localStorage.setItem("documents", JSON.stringify(parsedDocuments))
}

function deleteToken() {
    const {key} = getState();
    const documents = localStorage.getItem("documents");
    if (!documents) return;
    const parsedDocuments = JSON.parse(documents);
    delete parsedDocuments[key]
    localStorage.setItem("documents", JSON.stringify(parsedDocuments));
}

function updateCode(state) {
    const {mode, content} = state;

    const codeElement = document.querySelector("#code");
    const codeEditElement = document.querySelector("#code-edit");
    const codeViewElement = document.querySelector("#code-view");

    if (mode === "view") {
        codeEditElement.style.display = "none";
        codeEditElement.value = "";
        codeViewElement.innerText = content;
        codeElement.style.display = "block";
        highlightCode(state);
        return;
    }
    codeEditElement.value = content;
    codeEditElement.style.display = "block";
    codeViewElement.innerText = "";
    codeElement.style.display = "none";
}

function updatePage(state) {
    const {key, mode, content} = state;
    const token = getDocumentToken(key);
    // update page title
    if (key) {
        document.title = `gobin - ${key}`;
    } else {
        document.title = "gobin";
    }

    const saveButton = document.querySelector("#save");
    const editButton = document.querySelector("#edit");
    const deleteButton = document.querySelector("#delete");
    const copyButton = document.querySelector("#copy");
    const rawButton = document.querySelector("#raw");
    const shareButton = document.querySelector("#share");
    if (mode === "view") {
        saveButton.disabled = true;
        saveButton.style.display = "none";
        editButton.disabled = false;
        editButton.style.display = "block";
        deleteButton.disabled = !token;
        copyButton.disabled = false;
        rawButton.disabled = false;
        shareButton.disabled = false;
        return
    }
    saveButton.disabled = content === "";
    saveButton.style.display = "block";
    editButton.disabled = true;
    editButton.style.display = "none";
    deleteButton.disabled = true;
    copyButton.disabled = true;
    rawButton.disabled = true;
    shareButton.disabled = true;
}

function highlightCode(state) {
    const {content, language} = state;
    let result;
    if (language && language !== "auto") {
        result = hljs.highlight(content, {
            language: language, ignoreIllegals: true
        });
    } else {
        result = hljs.highlightAuto(content);
    }
    if (result.language === undefined) {
        result.language = "plaintext";
    }

    if (result.language !== language) {
        state.language = result.language;
    }

    const codeViewElement = document.querySelector("#code-view");
    codeViewElement.innerHTML = result.value;
    codeViewElement.className = "hljs language-" + result.language;

    document.querySelector("#language").value = result.language;

    if (result.value) {
        hljs.initLineNumbersOnLoad({singleLine: true});
    }
}