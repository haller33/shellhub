import { Module } from "vuex";
import * as apiAuth from "../api/auth";
import { IUserLogin } from "@/interfaces/IUserLogin";
import { State } from "..";

export interface AuthState {
  status: string;
  token: string;
  user: string;
  name: string;
  tenant: string;
  email: string;
  id: string;
  role: string;
  secret: string;
  link_mfa: string;
  mfa: {
    enable: boolean,
    validate: boolean,
  },
  recoveryCodes: Array<number>,
  showCongratulations: boolean,
  showRecoveryModal: boolean,
}
export const auth: Module<AuthState, State> = {
  namespaced: true,
  state: {
    status: "",
    token: localStorage.getItem("token") || "",
    user: localStorage.getItem("user") || "",
    name: localStorage.getItem("name") || "",
    tenant: localStorage.getItem("tenant") || "",
    email: localStorage.getItem("email") || "",
    id: localStorage.getItem("id") || "",
    role: localStorage.getItem("role") || "",
    secret: "",
    link_mfa: "",
    mfa: {
      enable: false,
      validate: false,
    },
    recoveryCodes: [],
    showCongratulations: false,
    showRecoveryModal: false,
  },

  getters: {
    isLoggedIn: (state) => !!state.token,
    authStatus: (state) => state.status,
    stateToken: (state) => state.token,
    currentUser: (state) => state.user,
    currentName: (state) => state.name,
    tenant: (state) => state.tenant,
    email: (state) => state.email,
    id: (state) => state.id,
    role: (state) => state.role,
    secret: (state) => state.secret,
    link_mfa: (state) => state.link_mfa,
    mfaStatus: (state) => state.mfa,
    isMfa: (state) => state.mfa.enable,
    isValidatedMfa: (state) => state.mfa.validate,
    recoveryCodes: (state) => state.recoveryCodes,
    showCongratulationsModal: (state) => state.showCongratulations,
    showRecoveryModal: (state) => state.showRecoveryModal,
  },

  mutations: {
    authRequest(state) {
      state.status = "loading";
    },

    showCongratulationsModal(state) {
      state.showCongratulations = !state.showCongratulations;
    },

    mfaEnabled(state, data) {
      state.mfa.enable = data;
    },

    mfaStatus(state, data) {
      state.mfa = data;
    },

    mfaToken(state, data) {
      state.token = data;
    },

    authSuccess(state, data) {
      state.status = "success";
      state.token = data.token;
      state.user = data.user;
      state.name = data.name;
      state.tenant = data.tenant;
      state.email = data.email;
      state.id = data.id;
      state.role = data.role;
      state.mfa = data.mfa;
    },

    authError(state) {
      state.status = "error";
    },

    logout(state) {
      state.status = "";
      state.token = "";
      state.name = "";
      state.user = "";
      state.tenant = "";
      state.email = "";
      state.role = "";
      state.mfa.enable = false;
    },

    changeData(state, data) {
      state.name = data.name;
      state.user = data.username;
      state.email = data.email;
    },

    mfaGenerateInfo(state, data) {
      state.link_mfa = data.link;
      state.secret = data.secret;
      state.recoveryCodes = data.codes;
    },

    userInfo(state, data) {
      state.link_mfa = data.link;
      state.secret = data.secret;
      state.recoveryCodes = data.codes;
      state.mfa = data.mfa;
      state.token = data.token;
      state.user = data.user;
      state.name = data.name;
      state.tenant = data.tenant;
      state.email = data.email;
      state.id = data.id;
      state.role = data.role;
      state.mfa = data.mfa;
    },

    accountRecoveryHelper(state) {
      state.showRecoveryModal = !state.showRecoveryModal;
    },
  },

  actions: {
    async login(context, user: IUserLogin) {
      context.commit("authRequest");

      try {
        const resp = await apiAuth.login(user);

        localStorage.setItem("token", resp.data.token || "");
        localStorage.setItem("user", resp.data.user || "");
        localStorage.setItem("name", resp.data.name || "");
        localStorage.setItem("tenant", resp.data.tenant || "");
        localStorage.setItem("email", resp.data.email || "");
        localStorage.setItem("id", resp.data.id || "");
        localStorage.setItem("namespacesWelcome", JSON.stringify({}));
        localStorage.setItem("role", resp.data.role || "");
        localStorage.setItem("mfa", resp.data.mfa?.enable ? "true" : "false");
        context.commit("authSuccess", resp.data);
      } catch (error) {
        context.commit("authError");
        throw error;
      }
    },

    async loginToken(context, token) {
      context.commit("authRequest");

      localStorage.setItem("token", token);

      try {
        const resp = await apiAuth.info();

        localStorage.setItem("token", resp.data.token || "");
        localStorage.setItem("user", resp.data.user ?? "");
        localStorage.setItem("name", resp.data.name ?? "");
        localStorage.setItem("tenant", resp.data.tenant ?? "");
        localStorage.setItem("id", resp.data.id ?? "");
        localStorage.setItem("email", resp.data.email ?? "");
        localStorage.setItem("namespacesWelcome", JSON.stringify({}));
        localStorage.setItem("role", resp.data.role ?? "");
        localStorage.setItem("mfa", resp.data.mfa?.enable ? "true" : "false");
        context.commit("authSuccess", resp.data);
      } catch (error) {
        context.commit("authError");
        throw error;
      }
    },

    async disableMfa(context) {
      try {
        await apiAuth.disableMfa();
        context.commit("mfaEnabled", false);
        localStorage.setItem("mfa", "false");
      } catch (error) {
        context.commit("authError");
        throw error;
      }
    },

    async enableMfa(context, data) {
      try {
        const resp = await apiAuth.enableMFA(data);

        if (resp.status === 200) {
          context.commit("mfaToken", resp.data.token);
          localStorage.setItem("token", resp.data.token || "");
          localStorage.setItem("mfa", "true");
          context.commit("mfaEnabled", true);
          context.commit("showCongratulationsModal");
        }
      } catch (error) {
        context.commit("authError");
        throw error;
      }
    },

    async validateMfa(context, data) {
      try {
        const resp = await apiAuth.validateMFA(data);

        if (resp.status === 200) {
          localStorage.setItem("token", resp.data.token || "");
          context.commit("mfaToken", resp.data.token);
        }
      } catch (error) {
        context.commit("authError");
        throw error;
      }
    },

    async generateMfa(context) {
      try {
        const resp = await apiAuth.generateMfa();
        if (resp.status === 200) {
          context.commit("mfaGenerateInfo", resp.data);
        }
      } catch (error) {
        context.commit("authError");
        throw error;
      }
    },

    async getUserInfo(context) {
      try {
        const resp = await apiAuth.info();
        if (resp.status === 200) {
          context.commit("userInfo", resp.data);
        }
      } catch (error) {
        context.commit("authError");
        throw error;
      }
    },

    async recoverLoginMfa(context, data) {
      try {
        const resp = await apiAuth.validateRecoveryCodes(data);
        if (resp.status === 200) {
          localStorage.setItem("token", resp.data.token || "");
          context.commit("mfaToken", resp.data.token);
          context.commit("accountRecoveryHelper");
        }
      } catch (error) {
        context.commit("authError");
        throw error;
      }
    },

    logout(context) {
      context.commit("logout");
      localStorage.removeItem("token");
      localStorage.removeItem("user");
      localStorage.removeItem("tenant");
      localStorage.removeItem("namespacesWelcome");
      localStorage.removeItem("noNamespace");
      localStorage.removeItem("email");
      localStorage.removeItem("id");
      localStorage.removeItem("name");
      localStorage.removeItem("role");
      localStorage.removeItem("mfa");
    },

    changeUserData(context, data) {
      localStorage.setItem("name", data.name);
      localStorage.setItem("user", data.username);
      localStorage.setItem("email", data.email);
      context.commit("changeData", data);
    },

    setShowWelcomeScreen(context, tenantID: string) {
      localStorage.setItem("namespacesWelcome", JSON.stringify(
        Object.assign(
          JSON.parse(localStorage.getItem("namespacesWelcome") || "") || {},
          { ...{ [tenantID]: true } },
        ),
      ));
    },
  },
};
