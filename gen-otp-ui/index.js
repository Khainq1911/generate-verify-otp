const inputs = document.querySelectorAll(".value");
const verify_btn = document.querySelector(".verify-btn");
const toggle_pass = document.querySelector(".pass-btn");
const pass_text = document.querySelector(".pass-text")

inputs[0].focus();

inputs.forEach((input, index) => {
  input.addEventListener("input", (e) => {
    let value = e.target.value;
    if (value && inputs.length - 1 > index) {
      inputs[index + 1].focus();
    }
  });
  input.addEventListener("keydown", (e) => {
    if (e.key === "Backspace" && index > 0 && !input.value) {
      inputs[index - 1].focus();
    }
  });
});

const checkOtp = () => {
  const otp = Array.from(inputs)
    .map((item, index) => {
      if (!item.value) {
        inputs[index].focus();
        throw new Error("Incomplete OTP");
      }
      return item.value;
    })
    .join("");

  return otp;
};

async function getData(otp) {
  const url = "http://localhost:3000/verify-otp";
  verify_btn.disabled = true;
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ otp: otp }),
    });
    if (!response.ok) {
      throw new Error(`Response status: ${response.status}`);
    }
    window.alert("OTP verified successful!");
  } catch (error) {
    window.alert("OTP verified fail!");
    console.error(error.message);
  }
}

const handleSubmit = () => {
  const otp = checkOtp();
  getData(otp);
  verify_btn.disabled = false;
};

verify_btn.addEventListener("click", handleSubmit);

async function getPassword() {
  const url = "http://localhost:3000/password";
  try {
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`Response status: ${response.status}`);
    }

    const json = await response.json();
    pass_text.innerHTML = json.data.Password
  } catch (error) {
    console.error(error.message);
  }
}
getPassword()

toggle_pass.addEventListener("click", () => {
  pass_text.classList.toggle("hide");
  if(pass_text.className.includes("hide")){
    toggle_pass.innerHTML = "Show Current Password"
  }
  else{
     toggle_pass.innerHTML = "Hide Current Password"
  }
});